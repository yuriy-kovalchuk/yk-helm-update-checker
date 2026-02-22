package api

import (
	"encoding/json"
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
	start := time.Now()
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Prevent multiple concurrent scans
	if !h.mu.TryLock() {
		http.Error(w, "A scan is already in progress. Please wait.", http.StatusTooManyRequests)
		return
	}
	defer h.mu.Unlock()

	metrics.ScansTotal.Inc()
	defer func() {
		metrics.ScanDuration.Observe(time.Since(start).Seconds())
	}()

	repoURL := strings.TrimSpace(r.URL.Query().Get("repo"))
	if repoURL == "" {
		repoURL = h.DefaultRepo
	}

	// 1. Sanitize Repo URL
	if len(repoURL) > 2048 {
		http.Error(w, "Repository URL is too long", http.StatusBadRequest)
		return
	}
	if strings.HasPrefix(repoURL, "-") {
		http.Error(w, "Invalid repository URL: cannot start with '-'", http.StatusBadRequest)
		return
	}

	// Support for standard URLs and Git SCP-like syntax
	u, err := url.Parse(repoURL)
	if err != nil {
		// url.Parse often fails on git@github.com:repo.git due to the colon
		if !strings.HasPrefix(repoURL, "git@") {
			http.Error(w, "Invalid repository URL format", http.StatusBadRequest)
			return
		}
	} else if u.Scheme != "" {
		// If a scheme is present, validate it
		allowed := map[string]bool{"http": true, "https": true, "git": true, "ssh": true}
		if !allowed[u.Scheme] {
			http.Error(w, "Invalid repository URL scheme. Allowed: http, https, git, ssh", http.StatusBadRequest)
			return
		}
	} else {
		// No scheme. Allow if it's a git@ address or a local path (if it exists)
		if !strings.HasPrefix(repoURL, "git@") {
			// Check if it's a local path for safety, but for now we'll allow it 
			// if it doesn't look like a flag or an exploit attempt.
			if strings.Contains(repoURL, ":") && !strings.Contains(repoURL, "/") {
				// Likely a malformed attempt or something else
				http.Error(w, "Invalid repository URL format", http.StatusBadRequest)
				return
			}
		}
	}

	subPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if subPath == "" {
		subPath = h.DefaultSubPath
	}

	// 2. Sanitize Path (Prevent Traversal and Injection)
	if len(subPath) > 1024 {
		http.Error(w, "Scan path is too long", http.StatusBadRequest)
		return
	}
	cleanPath := filepath.Clean(subPath)
	if strings.Contains(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") || strings.HasPrefix(cleanPath, "\\") {
		http.Error(w, "Invalid path: must be relative and cannot contain '..'", http.StatusBadRequest)
		return
	}

	// Restrict path to safe characters
	safePathRegex := regexp.MustCompile(`^[a-zA-Z0-9\.\-_/]*$`)
	if !safePathRegex.MatchString(cleanPath) {
		http.Error(w, "Invalid path: contains unsafe characters", http.StatusBadRequest)
		return
	}
	subPath = cleanPath

	ut := helm.UpdateType(r.URL.Query().Get("update_type"))
	if ut == "" {
		ut = helm.UpdateType(h.DefaultType)
	}

	// 3. Validate Update Type
	switch ut {
	case helm.UpdateAll, helm.UpdateMajor, helm.UpdateMinor, helm.UpdatePatch:
		// Valid
	default:
		http.Error(w, "Invalid update type", http.StatusBadRequest)
		return
	}

	tempDir, err := os.MkdirTemp("", "yk-api-*")
	if err != nil {
		metrics.ErrorsTotal.WithLabelValues("temp_dir").Inc()
		http.Error(w, "An internal error occurred", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	if err := github.DownloadRepo(repoURL, tempDir); err != nil {
		metrics.ErrorsTotal.WithLabelValues("clone").Inc()
		slog.Error("Failed to download repo", "url", repoURL, "error", err)
		http.Error(w, "Failed to download repository. Please check the URL and your permissions.", http.StatusInternalServerError)
		return
	}

	searchPath := filepath.Join(tempDir, subPath)
	chartPaths, err := helm.FindCharts(searchPath)
	if err != nil {
		metrics.ErrorsTotal.WithLabelValues("scan").Inc()
		slog.Error("Failed to scan charts", "path", searchPath, "error", err)
		http.Error(w, "An error occurred while scanning the repository for charts.", http.StatusInternalServerError)
		return
	}

	if len(chartPaths) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]models.UpdateResult{})
		return
	}

	var allResults []models.UpdateResult
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)

	// Reset old metrics before recording new ones for this repo
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

	// Sort results by Chart Name, then by Dependency name
	sort.Slice(allResults, func(i, j int) bool {
		if allResults[i].ChartName != allResults[j].ChartName {
			return allResults[i].ChartName < allResults[j].ChartName
		}
		return allResults[i].Dependency < allResults[j].Dependency
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(allResults)
}
