package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ScansTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "helm_update_checker_scans_total",
		Help: "The total number of repository scans performed",
	})

	UpdatesFound = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "helm_update_checker_updates_available",
		Help: "Indicates if an update is available (1) or not (0)",
	}, []string{"chart", "dependency", "current_version", "latest_version", "repo_type"})

	ScanDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "helm_update_checker_scan_duration_seconds",
		Help:    "Duration of the repository scan in seconds",
		Buckets: prometheus.DefBuckets,
	})

	ErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "helm_update_checker_errors_total",
		Help: "Total number of errors encountered during operation",
	}, []string{"type"})
)
