package server

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"time"
	"yk-update-checker/internal/api"
	"yk-update-checker/internal/helm"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// UI is the embedded filesystem containing static assets.
//
//go:embed all:web/templates all:web/static
var UI embed.FS

type Config struct {
	Port         string
	DefaultRepo  string
	SubPath      string
	UpdateType   string
	ScanInterval time.Duration
}

type Server struct {
	cfg     Config
	handler *api.UpdateHandler
}

func New(cfg Config) *Server {
	return &Server{
		cfg: cfg,
		handler: &api.UpdateHandler{
			DefaultRepo:    cfg.DefaultRepo,
			DefaultSubPath: cfg.SubPath,
			DefaultType:    cfg.UpdateType,
			Templates:      UI,
		},
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Routes
	mux.HandleFunc("/", s.handler.Index)
	mux.HandleFunc("/api/check", s.handler.CheckUpdates)
	mux.Handle("/metrics", promhttp.Handler())

	// Static files
	staticFS, _ := fs.Sub(UI, "web/static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Background Scanner
	if s.cfg.ScanInterval > 0 && s.cfg.DefaultRepo != "" {
		go s.startBackgroundScanner()
	}

	// Middleware (Logging)
	wrappedMux := s.loggingMiddleware(mux)

	slog.Info("Starting web server", "port", s.cfg.Port, "scan_interval", s.cfg.ScanInterval)
	return http.ListenAndServe(":"+s.cfg.Port, wrappedMux)
}

func (s *Server) startBackgroundScanner() {
	slog.Info("Starting background scanner", "interval", s.cfg.ScanInterval)
	ticker := time.NewTicker(s.cfg.ScanInterval)
	defer ticker.Stop()

	// Run first scan immediately
	s.handler.PerformScan(s.cfg.DefaultRepo, s.cfg.SubPath, helm.UpdateType(s.cfg.UpdateType))

	for range ticker.C {
		slog.Debug("Running scheduled scan")
		_, err := s.handler.PerformScan(s.cfg.DefaultRepo, s.cfg.SubPath, helm.UpdateType(s.cfg.UpdateType))
		if err != nil {
			slog.Error("Scheduled scan failed", "error", err)
		}
	}
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("Request received", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
