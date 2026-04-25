package scan

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/extractor"
	"github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/version"
)

// Result is one dependency check outcome, matching the UI columns:
// SOURCE | CHART | DEPENDENCY | TYPE | PROTOCOL | CURRENT | LATEST STABLE | SCOPE
type Result struct {
	Source          string    `json:"source"`
	Chart           string    `json:"chart"`
	Dependency      string    `json:"dependency"`
	Type            string    `json:"type"`
	Protocol        string    `json:"protocol"`
	CurrentVersion  string    `json:"current_version"`
	LatestVersion   string    `json:"latest_version"`
	Scope           string    `json:"scope"`
	UpdateAvailable bool      `json:"update_available"`
	CheckedAt       time.Time `json:"checked_at"`
}

// Scanner walks a directory tree, dispatches files to registered Extractors,
// and calls the version engine for each discovered ChartRef.
//
// newExtractors is called once per ScanDir invocation so that stateful
// extractors (e.g. FluxCD) receive a fresh instance for each repo and
// concurrent ScanDir calls never share mutable extractor state.
type Scanner struct {
	newExtractors  func() []extractor.Extractor
	scope          version.Scope
	parallelChecks int
}

func NewScanner(newExtractors func() []extractor.Extractor, scope version.Scope, parallelChecks int) *Scanner {
	return &Scanner{newExtractors: newExtractors, scope: scope, parallelChecks: parallelChecks}
}

// pendingCheck holds everything needed to perform one version lookup.
type pendingCheck struct {
	source string
	chart  string
	exType string
	ref    extractor.ChartRef
}

// ScanDir walks root and returns one Result per chart dependency found.
// It performs two streaming passes over the YAML files so that no more than
// one file's content is held in memory at a time.
// Cancelling ctx aborts in-flight version checks.
func (s *Scanner) ScanDir(ctx context.Context, source, root string) []Result {
	extractors := s.newExtractors()

	// walkYAML calls fn for each .yaml/.yml file under root, one at a time.
	walkYAML := func(fn func(path string, content []byte)) {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".yaml" && ext != ".yml" {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			fn(path, content)
			return nil
		})
	}

	// ── Pre-pass: stream each file through contextual extractors ──────────
	var contextuals []extractor.Contextual
	for _, ex := range extractors {
		if c, ok := ex.(extractor.Contextual); ok {
			contextuals = append(contextuals, c)
		}
	}
	if len(contextuals) > 0 {
		walkYAML(func(path string, content []byte) {
			for _, c := range contextuals {
				if err := c.PrepareFile(path, content); err != nil {
					slog.Warn("extractor prepare failed", "type", c.Type(), "file", path, "error", err)
				}
			}
		})
	}

	// ── Extract all chart refs (sequential; YAML parsing only) ────────────
	var pending []pendingCheck
	walkYAML(func(path string, content []byte) {
		for _, ex := range extractors {
			if !ex.Match(path, content) {
				continue
			}
			chartName, refs, err := ex.Extract(path, content)
			if err != nil {
				slog.Warn("extract failed", "file", path, "type", ex.Type(), "error", err)
				continue
			}
			for _, ref := range refs {
				pending = append(pending, pendingCheck{source: source, chart: chartName, exType: ex.Type(), ref: ref})
			}
		}
	})

	// ── Check versions concurrently (network I/O) ─────────────────────────
	var (
		results []Result
		mu      sync.Mutex
	)
	runConcurrent(ctx, pending, s.parallelChecks, func(ctx context.Context, p pendingCheck) {
		latest, err := version.Latest(ctx, p.ref.Protocol, p.ref.Repository, p.ref.Name, p.ref.CurrentVersion, s.scope)
		if err != nil {
			slog.Debug("version check failed", "dep", p.ref.Name, "error", err)
			latest = ""
		}

		// ref.Chart overrides the outer chartName for multi-doc files
		// where each document has its own logical owner (e.g. HelmRelease name).
		chart := p.ref.Chart
		if chart == "" {
			chart = p.chart
		}

		mu.Lock()
		results = append(results, Result{
			Source:          p.source,
			Chart:           chart,
			Dependency:      p.ref.Name,
			Type:            p.exType,
			Protocol:        p.ref.Protocol,
			CurrentVersion:  p.ref.CurrentVersion,
			LatestVersion:   latest,
			Scope:           string(s.scope),
			UpdateAvailable: isNewer(latest, p.ref.CurrentVersion),
			CheckedAt:       time.Now(),
		})
		mu.Unlock()
	})

	return results
}

// isNewer reports whether latest represents a strictly greater version than
// current. Semver comparison is used when both strings are valid semver so
// that tags differing only by a "v" prefix (e.g. "1.0.0" vs "v1.0.0") are
// not falsely reported as updates.
func isNewer(latest, current string) bool {
	if latest == "" {
		return false
	}
	l, err1 := semver.NewVersion(latest)
	c, err2 := semver.NewVersion(current)
	if err1 != nil || err2 != nil {
		return latest != current
	}
	return l.GreaterThan(c)
}
