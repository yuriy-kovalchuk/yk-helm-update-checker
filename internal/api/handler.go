package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"yk-update-checker/internal/github"
	"yk-update-checker/internal/helm"
	"yk-update-checker/internal/metrics"
	"yk-update-checker/internal/models"
	"yk-update-checker/internal/version"
)

type UpdateHandler struct {
	DefaultRepo    string
	DefaultSubPath string
	DefaultType    string
	Templates      fs.FS
	mu             sync.Mutex
}

type IndexData struct {
	RepoURL       string
	SubPath       string
	UpdateType    string
	IsDefaultRepo bool
	Version       string
}

func (h *UpdateHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	tmpl, err := template.ParseFS(h.Templates, "web/templates/index.html")
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	data := IndexData{
		RepoURL:       h.DefaultRepo,
		SubPath:       h.DefaultSubPath,
		UpdateType:    h.DefaultType,
		IsDefaultRepo: h.DefaultRepo != "",
		Version:       version.Version,
	}

	tmpl.Execute(w, data)
}

func (h *UpdateHandler) CheckUpdates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	repoURL := strings.TrimSpace(r.URL.Query().Get("repo"))
	if repoURL == "" {
		repoURL = h.DefaultRepo
	}
	subPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if subPath == "" {
		subPath = h.DefaultSubPath
	}
	ut := helm.UpdateType(r.URL.Query().Get("update_type"))
	if ut == "" {
		ut = helm.UpdateType(h.DefaultType)
	}

	results, err := h.PerformScan(repoURL, subPath, ut)
	if err != nil {
		if err.Error() == "busy" {
			http.Error(w, "A scan is already in progress. Please wait.", http.StatusTooManyRequests)
			return
		}
		if strings.Contains(err.Error(), "Invalid") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// PerformScan executes the full scan lifecycle and updates metrics.
func (h *UpdateHandler) PerformScan(repoURL, subPath string, ut helm.UpdateType) ([]models.UpdateResult, error) {
	start := time.Now()

	// Prevent multiple concurrent scans
	if !h.mu.TryLock() {
		return nil, fmt.Errorf("busy")
	}
	defer h.mu.Unlock()

	metrics.ScansTotal.Inc()
	defer func() {
		metrics.ScanDuration.Observe(time.Since(start).Seconds())
	}()

	// 1. Sanitize Repo URL
	if len(repoURL) > 2048 {
		return nil, fmt.Errorf("Invalid repository URL: too long")
	}
	if strings.HasPrefix(repoURL, "-") {
		return nil, fmt.Errorf("Invalid repository URL: cannot start with '-'")
	}
	if u, err := url.Parse(repoURL); err == nil {
		if u.Scheme != "" && u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "git" && u.Scheme != "ssh" {
			if !strings.HasPrefix(repoURL, "git@") {
				return nil, fmt.Errorf("Invalid repository URL scheme. Allowed: http, https, git, ssh")
			}
		}
	} else if !strings.HasPrefix(repoURL, "git@") {
		return nil, fmt.Errorf("Invalid repository URL format")
	}

	// 2. Sanitize Path
	cleanPath := filepath.Clean(subPath)
	if strings.Contains(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") || strings.HasPrefix(cleanPath, "\\") {
		return nil, fmt.Errorf("Invalid path: must be relative and cannot contain '..'")
	}
	safePathRegex := regexp.MustCompile(`^[a-zA-Z0-9\.\-_/]*$`)
	if !safePathRegex.MatchString(cleanPath) {
		return nil, fmt.Errorf("Invalid path: contains unsafe characters")
	}

	// 3. Validate Update Type
	switch ut {
	case helm.UpdateAll, helm.UpdateMajor, helm.UpdateMinor, helm.UpdatePatch:
	default:
		return nil, fmt.Errorf("Invalid update type")
	}

	tempDir, err := os.MkdirTemp("", "yk-api-*")
	if err != nil {
		metrics.ErrorsTotal.WithLabelValues("temp_dir").Inc()
		return nil, fmt.Errorf("An internal error occurred")
	}
	defer os.RemoveAll(tempDir)

	slog.Info("Performing scan", "repo", repoURL, "path", cleanPath)
	if err := github.DownloadRepo(repoURL, tempDir); err != nil {
		metrics.ErrorsTotal.WithLabelValues("clone").Inc()
		slog.Error("Failed to download repo", "url", repoURL, "error", err)
		return nil, fmt.Errorf("Failed to download repository")
	}

	searchPath := filepath.Join(tempDir, cleanPath)
	chartPaths, err := helm.FindCharts(searchPath)
	if err != nil {
		metrics.ErrorsTotal.WithLabelValues("scan").Inc()
		return nil, fmt.Errorf("An error occurred while scanning for charts")
	}

	if len(chartPaths) == 0 {
		return []models.UpdateResult{}, nil
	}

	var allResults []models.UpdateResult
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)

	metrics.UpdatesFound.Reset()

	for _, path := range chartPaths {
		chart, err := helm.ParseChart(path)
		if err != nil {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(c *models.Chart) {
			defer wg.Done()
			defer func() { <-sem }()
			res := helm.CheckUpdates(c, ut)
			mu.Lock()
			allResults = append(allResults, res...)
			for _, r := range res {
				metrics.UpdatesFound.WithLabelValues(r.ChartName, r.Dependency, r.CurrentVersion, r.LatestVersion, r.RepoType).Set(1)
			}
			mu.Unlock()
		}(chart)
	}
	wg.Wait()

	sort.Slice(allResults, func(i, j int) bool {
		if allResults[i].ChartName != allResults[j].ChartName {
			return allResults[i].ChartName < allResults[j].ChartName
		}
		return allResults[i].Dependency < allResults[j].Dependency
	})

	return allResults, nil
}
