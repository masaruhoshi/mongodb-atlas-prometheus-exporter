package main

import (
	"fmt"
	"runtime"

	"github.com/masaruhoshi/mongodb-atlas-prometheus-exporter/version"
	"github.com/prometheus/client_golang/prometheus"
)

const metricsNS = "mongodb_atlas"

var (
	// Metrics about the exporter itself.
	buildInfo = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: metricsNS,
			Name:      "build_info",
			Help: fmt.Sprintf(
				"A metric with a constant '1' value labeled by version, revision and goversion from which %s was built.",
				metricsNS,
			),
			ConstLabels: prometheus.Labels{
				"version":   version.Version,
				"revision":  version.Revision,
				"goversion": runtime.Version(),
			},
		},
		func() float64 { return 1 },
	)
	exporterDurationSummary = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Namespace: metricsNS,
			Name:      "collection_duration_quantiles_seconds",
			Help:      "Summary duration of collections by the MongoDB Atlas Exporter",
			//nolint:gomnd
			Objectives: map[float64]float64{0.1: 0.05, 0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
	)
	exporterDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: metricsNS,
			Name:      "collection_duration_seconds",
			Help:      "Duration of collections by the MongoDB Atlas Exporter",
			Buckets:   []float64{1, 2.5, 5, 8, 10, 15},
		},
	)
	exporterRequestErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: metricsNS,
			Name:      "request_errors_total",
			Help:      "Errors in requests to the MongoDB Atlas Exporter",
		},
	)
	exporterClientErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: metricsNS,
			Name:      "client_errors_total",
			Help:      "Errors with the MongoDB Atlas client",
		},
	)
)

func initExporterMetrics() {
	prometheus.MustRegister(buildInfo)
	prometheus.MustRegister(exporterDuration, exporterDurationSummary, exporterRequestErrors, exporterClientErrors)
}
