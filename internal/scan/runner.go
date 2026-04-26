package scan

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/extractor"
	"github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/version"
)

// RepoAuth holds credentials for a repository.
// Type selects the mechanism: "token", "basic", or "ssh".
// File variants (TokenFile, PasswordFile) point to files mounted from Secrets.
type RepoAuth struct {
	Type         string
	Token        string
	TokenFile    string
	Username     string
	Password     string
	PasswordFile string
	SSHKeyPath   string
}

// RepoTarget is a repository to scan with an optional sub-path and auth.
type RepoTarget struct {
	Name string
	URL  string
	Path string
	Auth RepoAuth
}

// cloneURL returns the repository URL with credentials embedded for HTTPS
// clones. SSH URLs are returned unchanged (auth handled via GIT_SSH_COMMAND).
// When inline values are absent, credentials are read from the file paths
// set by TokenFile / PasswordFile (mounted from Kubernetes Secrets).
func (rt RepoTarget) cloneURL() string {
	u, err := url.Parse(rt.URL)
	if err != nil || u.Scheme == "" {
		return rt.URL
	}
	switch rt.Auth.Type {
	case "token":
		tok := rt.Auth.Token
		if tok == "" && rt.Auth.TokenFile != "" {
			tok = readCredFile(rt.Auth.TokenFile)
		}
		if tok != "" {
			u.User = url.UserPassword("git", tok)
		}
	case "basic":
		pass := rt.Auth.Password
		if pass == "" && rt.Auth.PasswordFile != "" {
			pass = readCredFile(rt.Auth.PasswordFile)
		}
		if pass != "" {
			u.User = url.UserPassword(rt.Auth.Username, pass)
		}
	}
	return u.String()
}

func readCredFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("credential file unreadable", "path", path, "error", err)
		return ""
	}
	return strings.TrimSpace(string(data))
}

// authEnv returns the environment for git commands. It always sets
// GIT_TERMINAL_PROMPT=0 so git fails fast instead of hanging on missing
// credentials. SSH key auth is injected via GIT_SSH_COMMAND.
func (rt RepoTarget) authEnv() []string {
	env := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if rt.Auth.Type == "ssh" && rt.Auth.SSHKeyPath != "" {
		env = append(env,
			"GIT_SSH_COMMAND=ssh -i "+rt.Auth.SSHKeyPath+" -o StrictHostKeyChecking=no -o BatchMode=yes",
		)
	}
	return env
}

// Runner clones repositories and runs the Scanner on each one.
type Runner struct {
	repos          []RepoTarget
	newExtractors  func() []extractor.Extractor
	scope          version.Scope
	parallelChecks int
	gitCacheDir    string
}

// NewRunner creates a Runner. newExtractors is called once per repo scan so
// that stateful extractors receive a fresh instance per repository and
// concurrent scans never share mutable extractor state.
func NewRunner(repos []RepoTarget, newExtractors func() []extractor.Extractor, scope version.Scope, parallelChecks int, gitCacheDir string) *Runner {
	return &Runner{
		repos:          repos,
		newExtractors:  newExtractors,
		scope:          scope,
		parallelChecks: parallelChecks,
		gitCacheDir:    gitCacheDir,
	}
}

// Run syncs all repos, scans them, and returns the aggregated results.
// When gitCacheDir is empty a temporary directory is used and removed on
// return; when set the clones are kept there and subsequent runs do a fast
// fetch+reset instead of a full clone.
// Cancelling ctx aborts in-flight clones and version checks.
func (r *Runner) Run(ctx context.Context) ([]Result, error) {
	var (
		workDir string
		cleanup bool
	)
	if r.gitCacheDir != "" {
		if err := os.MkdirAll(r.gitCacheDir, 0o755); err != nil {
			return nil, fmt.Errorf("create git cache dir: %w", err)
		}
		workDir = r.gitCacheDir
	} else {
		dir, err := os.MkdirTemp("", "yk-scan-*")
		if err != nil {
			return nil, err
		}
		workDir = dir
		cleanup = true
	}
	if cleanup {
		defer os.RemoveAll(workDir)
	}

	cache := version.NewIndexCache()
	scanner := NewScanner(r.newExtractors, r.scope, r.parallelChecks, cache)

	var (
		results []Result
		mu      sync.Mutex
	)
	runConcurrent(ctx, r.repos, 5, func(ctx context.Context, rt RepoTarget) {
		dest := filepath.Join(workDir, safeName(rt.Name))
		slog.Info("syncing repo", "name", rt.Name, "url", rt.URL)
		if err := syncRepo(ctx, rt, dest); err != nil {
			slog.Error("sync failed", "repo", rt.Name, "error", err)
			return
		}

		scanPath := dest
		if rt.Path != "" {
			scanPath = filepath.Join(dest, rt.Path)
		}

		repoResults := scanner.ScanDir(ctx, rt.Name, scanPath)
		slog.Info("scan done", "repo", rt.Name, "results", len(repoResults))

		mu.Lock()
		results = append(results, repoResults...)
		mu.Unlock()
	})

	return results, nil
}

// syncRepo clones the repo if it doesn't exist yet, or does a fast
// fetch+reset if a clone is already present in dest.
func syncRepo(ctx context.Context, rt RepoTarget, dest string) error {
	if _, err := os.Stat(filepath.Join(dest, ".git")); err == nil {
		return fetchRepo(ctx, rt, dest)
	}
	return cloneRepo(ctx, rt, dest)
}

func cloneRepo(ctx context.Context, rt RepoTarget, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", "--single-branch", rt.cloneURL(), dest)
	cmd.Env = rt.authEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func fetchRepo(ctx context.Context, rt RepoTarget, dest string) error {
	fetch := exec.CommandContext(ctx, "git", "-C", dest, "fetch", "--depth=1", "origin")
	fetch.Env = rt.authEnv()
	if out, err := fetch.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	reset := exec.CommandContext(ctx, "git", "-C", dest, "reset", "--hard", "FETCH_HEAD")
	if out, err := reset.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// runConcurrent fans out fn across items using at most limit goroutines.
// It stops launching new goroutines when ctx is cancelled; already-running
// goroutines receive the cancelled context and are expected to return promptly.
func runConcurrent[T any](ctx context.Context, items []T, limit int, fn func(context.Context, T)) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, limit)
	for _, item := range items {
		if ctx.Err() != nil {
			break
		}
		item := item
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			fn(ctx, item)
		}()
	}
	wg.Wait()
}

// safeName converts s into a string safe for use as a directory name by
// replacing any character that is not a letter, digit, dash, dot, or
// underscore with a dash.
func safeName(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, s)
}
