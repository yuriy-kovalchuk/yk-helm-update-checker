// Package api provides the HTTP API server that manages the database.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/config"
	"github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/db"
	"github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/trigger"
)

// Server is the HTTP API server that owns the database.
type Server struct {
	db      *db.DB
	trigger *trigger.KubernetesTrigger
	port    string
}

// Config holds server configuration.
type Config struct {
	DB      *db.DB
	Trigger *trigger.KubernetesTrigger
	Port    string
}

// New creates a new API server.
func New(cfg *Config) *Server {
	return &Server{
		db:      cfg.DB,
		trigger: cfg.Trigger,
		port:    cfg.Port,
	}
}

const (
	maxRequestBody    = 4 << 20       // 4 MB
	stuckScanTimeout  = 2 * time.Hour
	stuckScanInterval = time.Minute
)

// Run starts the HTTP server and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()

	// Scanner endpoints
	mux.HandleFunc("POST /api/scans", s.createScan)
	mux.HandleFunc("POST /api/scans/{id}/results", s.addResults)
	mux.HandleFunc("PATCH /api/scans/{id}", s.updateScan)

	// Dashboard endpoints
	mux.HandleFunc("GET /api/scans", s.listScans)
	mux.HandleFunc("GET /api/results", s.getResults)
	mux.HandleFunc("GET /api/status", s.getStatus)
	mux.HandleFunc("POST /api/trigger", s.triggerScan)

	// Health
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /ready", s.health)

	srv := &http.Server{
		Addr: ":" + s.port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
			mux.ServeHTTP(w, r)
		}),
	}

	go func() {
		slog.Info("api server started", "addr", "http://localhost:"+s.port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
		}
	}()

	go func() {
		ticker := time.NewTicker(stuckScanInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n, err := s.db.FailStuckScans(stuckScanTimeout)
				if err != nil {
					slog.Error("fail stuck scans", "error", err)
				} else if n > 0 {
					slog.Warn("auto-failed stuck scans", "count", n)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down api server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}

// createScan creates a new scan record (called by scanner).
func (s *Server) createScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Scope   string `json:"scope"`
		Trigger string `json:"trigger"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	trig := db.ScanTriggerManual
	if req.Trigger == "scheduled" {
		trig = db.ScanTriggerScheduled
	}

	scanID, err := s.db.CreateScan(req.Scope, trig)
	if err != nil {
		slog.Error("create scan failed", "error", err)
		http.Error(w, "failed to create scan", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]int64{"id": scanID})
}

// addResults adds results to a scan (called by scanner).
func (s *Server) addResults(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid scan ID", http.StatusBadRequest)
		return
	}

	var results []db.Result
	if err := json.NewDecoder(r.Body).Decode(&results); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	for i := range results {
		results[i].ScanID = id
	}

	if err := s.db.InsertResults(results); err != nil {
		slog.Error("insert results failed", "error", err)
		http.Error(w, "failed to insert results", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]int{"count": len(results)})
}

// updateScan updates scan status (called by scanner to complete/fail).
func (s *Server) updateScan(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid scan ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Status      string `json:"status"`
		ResultCount int    `json:"result_count"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	switch req.Status {
	case "completed":
		if err := s.db.CompleteScan(id, req.ResultCount); err != nil {
			slog.Error("complete scan failed", "error", err)
			http.Error(w, "failed to complete scan", http.StatusInternalServerError)
			return
		}
	case "failed":
		if err := s.db.FailScan(id, req.Error); err != nil {
			slog.Error("fail scan failed", "error", err)
			http.Error(w, "failed to mark scan as failed", http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": req.Status})
}

// getResults returns results from the most recent completed scan.
func (s *Server) getResults(w http.ResponseWriter, _ *http.Request) {
	results, err := s.db.GetLatestResults()
	if err != nil {
		slog.Error("get results failed", "error", err)
		http.Error(w, "failed to get results", http.StatusInternalServerError)
		return
	}

	resp := make([]map[string]any, len(results))
	for i, r := range results {
		resp[i] = map[string]any{
			"source":           r.Source,
			"chart":            r.Chart,
			"dependency":       r.Dependency,
			"type":             r.Type,
			"protocol":         r.Protocol,
			"current_version":  r.CurrentVersion,
			"latest_version":   r.LatestVersion,
			"scope":            r.Scope,
			"update_available": r.UpdateAvailable,
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// getStatus returns current status (scanning, last scan, trigger availability).
func (s *Server) getStatus(w http.ResponseWriter, r *http.Request) {
	scanning, err := s.db.IsScanning()
	if err != nil {
		slog.Error("is scanning check failed", "error", err)
		http.Error(w, "failed to get status", http.StatusInternalServerError)
		return
	}
	triggerAvailable := s.trigger != nil && s.trigger.Available()

	resp := map[string]any{
		"scanning":          scanning,
		"trigger_available": triggerAvailable,
		"version":           config.Version,
	}

	if scan, err := s.db.LatestCompletedScan(); err == nil && scan != nil {
		if scan.CompletedAt != nil {
			resp["last_scan"] = scan.CompletedAt.UTC().Format(time.RFC3339)
		}
		resp["result_count"] = scan.ResultCount
	}

	writeJSON(w, http.StatusOK, resp)
}

// triggerScan creates a K8s Job to run a scan.
func (s *Server) triggerScan(w http.ResponseWriter, r *http.Request) {
	if s.trigger == nil || !s.trigger.Available() {
		http.Error(w, "trigger not available", http.StatusServiceUnavailable)
		return
	}

	jobName, err := s.trigger.Trigger(r.Context())
	if err != nil {
		slog.Error("trigger scan failed", "error", err)
		http.Error(w, "failed to trigger scan", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"job": jobName})
}

// listScans returns recent scan history (last 20).
func (s *Server) listScans(w http.ResponseWriter, _ *http.Request) {
	scans, err := s.db.ListScans(5)
	if err != nil {
		slog.Error("list scans failed", "error", err)
		http.Error(w, "failed to list scans", http.StatusInternalServerError)
		return
	}

	resp := make([]map[string]any, len(scans))
	for i, sc := range scans {
		m := map[string]any{
			"id":                sc.ID,
			"status":            string(sc.Status),
			"trigger":           string(sc.Trigger),
			"scope":             sc.Scope,
			"result_count":      sc.ResultCount,
			"updates_available": sc.UpdatesAvailable,
			"started_at":        sc.StartedAt.UTC().Format(time.RFC3339),
		}
		if sc.CompletedAt != nil {
			m["completed_at"] = sc.CompletedAt.UTC().Format(time.RFC3339)
			m["duration_s"] = sc.CompletedAt.Sub(sc.StartedAt).Seconds()
		}
		if sc.ErrorMessage != "" {
			m["error"] = sc.ErrorMessage
		}
		resp[i] = m
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("json encode failed", "error", err)
	}
}
