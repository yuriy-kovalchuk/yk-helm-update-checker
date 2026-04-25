package scan

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/extractor"
	"github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/version"
)

// RepoTarget is a repository to scan with an optional sub-path.
type RepoTarget struct {
	Name string
	URL  string
	Path string
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
		if err := syncRepo(ctx, rt.URL, dest); err != nil {
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
func syncRepo(ctx context.Context, url, dest string) error {
	if _, err := os.Stat(filepath.Join(dest, ".git")); err == nil {
		return fetchRepo(ctx, dest)
	}
	return cloneRepo(ctx, url, dest)
}

func cloneRepo(ctx context.Context, url, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", "--single-branch", url, dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func fetchRepo(ctx context.Context, dest string) error {
	fetch := exec.CommandContext(ctx, "git", "-C", dest, "fetch", "--depth=1", "origin")
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
