package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"yk-update-checker/internal/github"
	"yk-update-checker/internal/helm"
	"yk-update-checker/internal/models"
	"yk-update-checker/internal/server"
	"yk-update-checker/internal/version"
)

func main() {
	repoURL := flag.String("repo", "", "GitHub repository URL to scan")
	scanSubPath := flag.String("path", ".", "Path within the repository to scan for Helm charts")
	tempDir := flag.String("temp-dir", "", "Temporary directory to clone the repository (defaults to a system temp dir)")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	updateType := flag.String("update-type", "all", "Update type to check for: all, major, minor, patch")
	webMode := flag.Bool("web", false, "Start in web server mode")
	port := flag.String("port", "8080", "Port to run the web server on")
	flag.Parse()

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	slog.Info("Starting YK-UPDATE-CHECKER", "version", version.Version, "commit", version.Commit, "build_date", version.BuildDate)

	if *webMode {
		srv := server.New(server.Config{
			Port:        *port,
			DefaultRepo: *repoURL,
			SubPath:     *scanSubPath,
			UpdateType:  *updateType,
		})
		if err := srv.Start(); err != nil {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
		return
	}

	if *repoURL == "" {
		fmt.Println("Error: -repo flag is required in CLI mode")
		flag.Usage()
		os.Exit(1)
	}

	ut := helm.UpdateType(*updateType)
	switch ut {
	case helm.UpdateAll, helm.UpdateMajor, helm.UpdateMinor, helm.UpdatePatch:
		// Valid
	default:
		fmt.Printf("Error: invalid update-type '%s'. Valid types: all, major, minor, patch\n", *updateType)
		os.Exit(1)
	}

	workDir := *tempDir
	if workDir == "" {
		var err error
		workDir, err = os.MkdirTemp("", "yk-update-checker-*")
		if err != nil {
			slog.Error("Failed to create temp directory", "error", err)
			os.Exit(1)
		}
		defer os.RemoveAll(workDir)
	}

	slog.Info("Downloading repository", "url", *repoURL, "target", workDir)
	if err := github.DownloadRepo(*repoURL, workDir); err != nil {
		slog.Error("Failed to download repository", "error", err)
		os.Exit(1)
	}

	searchPath := filepath.Join(workDir, *scanSubPath)
	slog.Info("Scanning for Helm charts", "path", searchPath)
	chartPaths, err := helm.FindCharts(searchPath)
	if err != nil {
		slog.Error("Failed to find charts", "error", err)
		os.Exit(1)
	}

	slog.Info("Found charts", "count", len(chartPaths))

	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // Limit concurrency

	for _, path := range chartPaths {
		chart, err := helm.ParseChart(path)
		if err != nil {
			slog.Error("Failed to parse chart", "path", path, "error", err)
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(c *models.Chart) {
			defer wg.Done()
			defer func() { <-sem }()
			helm.CheckUpdates(c, ut)
		}(chart)
	}

	wg.Wait()
	slog.Info("Update check completed")
}
