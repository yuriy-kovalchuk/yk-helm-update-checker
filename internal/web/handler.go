package web

import (
	"embed"
	"log/slog"
	"net/http"
)

//go:embed ui
var uiFS embed.FS

// ServeUI serves the embedded index.html file.
func ServeUI(w http.ResponseWriter, _ *http.Request) {
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
