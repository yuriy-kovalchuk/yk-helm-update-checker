package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/config"
	"github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/extractor"
	"github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/scan"
	"github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/version"
	"github.com/yuriy-kovalchuk/yk-helm-update-checker/internal/web"
)

func main() {
	cfgPath   := flag.String("config",  "config.yaml", "path to config file")
	scopeFlag := flag.String("scope",   "",            "update scope: all, major, minor, patch (overrides config)")
	webMode   := flag.Bool("web",       false,         "start web server instead of CLI scan")
	port      := flag.String("port",    "8080",        "web server port")
	verbose   := flag.Bool("verbose",   false,         "enable debug logging")
	flag.Parse()

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fatal("load config: %v", err)
	}

	// --scope overrides the config file value; both fall back to "all".
	scope := cfg.UpdateType
	if *scopeFlag != "" {
		scope = *scopeFlag
	}

	if *webMode {
		startWeb(cfg, scope, *port)
		return
	}

	if len(cfg.Repos) == 0 {
		fatal("no repositories configured in %s", *cfgPath)
	}
	if err := runCLI(cfg, scope); err != nil {
		fatal("%v", err)
	}
}

func runCLI(cfg *config.Config, scopeStr string) error {
	sc := version.ParseScope(scopeStr)

	newExtractors := func() []extractor.Extractor {
		return []extractor.Extractor{extractor.NewHelmChart(), extractor.NewFluxCD()}
	}

	repos := make([]scan.RepoTarget, len(cfg.Repos))
	for i, r := range cfg.Repos {
		repos[i] = scan.RepoTarget{Name: r.Name, URL: r.URL, Path: r.Path}
	}

	runner := scan.NewRunner(repos, newExtractors, sc)
	slog.Info("starting scan", "repos", len(repos), "scope", sc)
	results, err := runner.Run(context.Background())
	if err != nil {
		return err
	}
	slog.Info("scan complete", "results", len(results))

	writeTable(os.Stdout, results)
	return nil
}

func writeTable(w io.Writer, results []scan.Result) {
	if len(results) == 0 {
		fmt.Fprintln(w, "No results found.")
		return
	}

	sorted := make([]scan.Result, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Source != sorted[j].Source {
			return sorted[i].Source < sorted[j].Source
		}
		if sorted[i].Chart != sorted[j].Chart {
			return sorted[i].Chart < sorted[j].Chart
		}
		return sorted[i].Dependency < sorted[j].Dependency
	})

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SOURCE\tCHART\tDEPENDENCY\tTYPE\tPROTOCOL\tCURRENT\tLATEST STABLE\tSCOPE")
	fmt.Fprintln(tw, "------\t-----\t----------\t----\t--------\t-------\t-------------\t-----")
	for _, r := range sorted {
		latest := r.LatestVersion
		if latest == "" {
			latest = "-"
		}
		marker := " "
		if r.UpdateAvailable {
			marker = "*"
		}
		fmt.Fprintf(tw, "%s%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			marker, r.Source, r.Chart, r.Dependency, r.Type,
			r.Protocol, r.CurrentVersion, latest, r.Scope,
		)
	}
	tw.Flush()
}

func startWeb(cfg *config.Config, scope, port string) {
	h := web.NewHandler(cfg, scope)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	addr := ":" + port
	slog.Info("web server started", "addr", "http://localhost"+addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fatal("server: %v", err)
	}
}

func fatal(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: %s\n", fmt.Sprintf(msg, args...))
	os.Exit(1)
}
