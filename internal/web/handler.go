package web

import (
	"context"
	"embed"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/config"
	"github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/extractor"
	"github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/scan"
	"github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/version"
)

//go:embed ui
var uiFS embed.FS

type statusResponse struct {
	Scanning bool      `json:"scanning"`
	LastScan time.Time `json:"last_scan"`
	Count    int       `json:"count"`
}

// Handler serves the web UI and JSON API.
type Handler struct {
	repos          []scan.RepoTarget
	scope          version.Scope
	parallelChecks int
	gitCacheDir    string
	scanning       atomic.Bool // guards against concurrent scans; CompareAndSwap used in startScan
	mu             sync.RWMutex
	results        []scan.Result
	lastScan       time.Time
	count          int
}

func NewHandler(cfg *config.Config, scopeStr string) *Handler {
	repos := make([]scan.RepoTarget, len(cfg.Repos))
	for i, r := range cfg.Repos {
		repos[i] = scan.RepoTarget{
			Name: r.Name,
			URL:  r.URL,
			Path: r.Path,
			Auth: scan.RepoAuth{
				Type:         r.Auth.Type,
				Token:        r.Auth.Token,
				TokenFile:    r.Auth.TokenFile,
				Username:     r.Auth.Username,
				Password:     r.Auth.Password,
				PasswordFile: r.Auth.PasswordFile,
				SSHKeyPath:   r.Auth.SSHKeyPath,
			},
		}
	}
	return &Handler{
		repos:          repos,
		scope:          version.ParseScope(scopeStr),
		parallelChecks: cfg.ParallelChecks,
		gitCacheDir:    cfg.GitCacheDir,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /", h.serveUI)
	mux.HandleFunc("POST /api/scan", h.startScan)
	mux.HandleFunc("GET /api/results", h.getResults)
	mux.HandleFunc("GET /api/status", h.getStatus)
	mux.HandleFunc("GET /api/config", h.getConfig)
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /ready", h.ready)
}

func (h *Handler) serveUI(w http.ResponseWriter, _ *http.Request) {
	data, err := uiFS.ReadFile("ui/index.html")
	if err != nil {
		http.Error(w, "UI not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(data); err != nil {
		slog.Error("serve UI write failed", "error", err)
	}
}

func (h *Handler) startScan(w http.ResponseWriter, _ *http.Request) {
	// CompareAndSwap atomically checks-and-sets scanning, eliminating the
	// TOCTOU window that existed when check and set were separate operations.
	if !h.scanning.CompareAndSwap(false, true) {
		writeJSON(w, map[string]string{"status": "already scanning"})
		return
	}
	go h.runScan()
	writeJSON(w, map[string]string{"status": "started"})
}

func (h *Handler) runScan() {
	defer h.scanning.Store(false)

	newExtractors := func() []extractor.Extractor {
		return []extractor.Extractor{extractor.NewHelmChart(), extractor.NewFluxCD()}
	}
	runner := scan.NewRunner(h.repos, newExtractors, h.scope, h.parallelChecks, h.gitCacheDir)

	slog.Info("web scan started", "repos", len(h.repos))
	results, err := runner.Run(context.Background())
	if err != nil {
		slog.Error("scan failed", "error", err)
		return
	}
	slog.Info("web scan complete", "results", len(results))

	h.mu.Lock()
	h.results = results
	h.lastScan = time.Now()
	h.count = len(results)
	h.mu.Unlock()
}

func (h *Handler) getResults(w http.ResponseWriter, _ *http.Request) {
	h.mu.RLock()
	results := h.results
	h.mu.RUnlock()

	if results == nil {
		results = []scan.Result{}
	}
	writeJSON(w, results)
}

func (h *Handler) getStatus(w http.ResponseWriter, _ *http.Request) {
	h.mu.RLock()
	s := statusResponse{
		Scanning: h.scanning.Load(),
		LastScan: h.lastScan,
		Count:    h.count,
	}
	h.mu.RUnlock()
	writeJSON(w, s)
}

func (h *Handler) getConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"version": config.Version,
		"commit":  config.Commit,
		"scope":   string(h.scope),
		"repos":   h.repos,
	})
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}

// ready returns 200 once the server is up and not mid-scan, 503 while a scan
// is running. Kubernetes uses this to hold traffic until the pod is stable.
func (h *Handler) ready(w http.ResponseWriter, _ *http.Request) {
	if h.scanning.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeJSON(w, map[string]string{"status": "scanning"})
		return
	}
	writeJSON(w, map[string]string{"status": "ready"})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("json encode failed", "error", err)
	}
}
