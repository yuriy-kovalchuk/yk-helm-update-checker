package scan

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	gogit "github.com/go-git/go-git/v5"
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
	repos         []RepoTarget
	newExtractors func() []extractor.Extractor
	scope         version.Scope
}

// NewRunner creates a Runner. newExtractors is called once per repo scan so
// that stateful extractors receive a fresh instance per repository and
// concurrent scans never share mutable extractor state.
func NewRunner(repos []RepoTarget, newExtractors func() []extractor.Extractor, scope version.Scope) *Runner {
	return &Runner{repos: repos, newExtractors: newExtractors, scope: scope}
}

// Run clones all repos into a temporary directory, scans them, and returns
// the aggregated results. The temp directory is removed on return.
// Cancelling ctx aborts in-flight clones and version checks.
func (r *Runner) Run(ctx context.Context) ([]Result, error) {
	workDir, err := os.MkdirTemp("", "yk-scan-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workDir)

	scanner := NewScanner(r.newExtractors, r.scope)

	var (
		results []Result
		mu      sync.Mutex
	)
	runConcurrent(ctx, r.repos, 5, func(ctx context.Context, rt RepoTarget) {
		dest := filepath.Join(workDir, safeName(rt.Name))
		slog.Info("cloning repo", "name", rt.Name, "url", rt.URL)
		if err := cloneRepo(ctx, rt.URL, dest); err != nil {
			slog.Error("clone failed", "repo", rt.Name, "error", err)
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

func cloneRepo(ctx context.Context, url, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	_, err := gogit.PlainCloneContext(ctx, dest, false, &gogit.CloneOptions{
		URL:   url,
		Depth: 1,
	})
	return err
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
