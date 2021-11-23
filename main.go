package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/alecthomas/kong"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/masaruhoshi/mongodb-atlas-prometheus-exporter/version"
	"github.com/mongodb-forks/digest"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"go.mongodb.org/atlas/mongodbatlas"
)

const prog = "mongodb-atlas-prometheus-exporter"

type GlobalFlags struct {
	AtlasPublicKey   string `required:"true" name:"atlas.api-public-key" help:"Atlas API public key" env:"ATLAS_PUBLIC_KEY"`
	AtlasPrivateKey  string `required:"true" name:"atlas.api-private-key" help:"Atlas API private key" env:"ATLAS_PRIVATE_KEY"`
	AtlasProjectId   string `required:"true" name:"atlas.project" help:"Atlas project (group) id"`
	WebListenAddress string `name:"web.listen-address" help:"Address to listen on for web interface and telemetry" default:":9139"`
	WebScrapePath    string `name:"web.scrape-path" help:"API metrics path" default:"/scrape"`
	WebTelemetryPath string `name:"web.telemetry-path" help:"Exporter metrics path" default:"/metrics"`
	LogLevel         string `name:"log.level" help:"Only log messages with the given severuty or above. Valid levels: [debug, info, warn, error, fatal]" enum:"debug,info,warn,error,fatal" default:"error"`
	Version          bool   `name:"version" help:"Show version and exit"`
}

func handler(w http.ResponseWriter, r *http.Request, logger log.Logger, client *mongodbatlas.Client, projectId string) {
	level.Debug(logger).Log("msg", "Starting scrape")

	start := time.Now()

	registry := prometheus.NewRegistry()
	collector := newCollector(r.Context(), logger, client, projectId)
	registry.MustRegister(collector)

	// Delegate http serving to Prometheus client library, which will call collector.Collect.
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)

	duration := time.Since(start).Seconds()
	exporterDurationSummary.Observe(duration)
	exporterDuration.Observe(duration)

	level.Debug(logger).Log("msg", "Finished scrape", "duration_seconds", duration)
}

func getAtlasClient(opts GlobalFlags, logger log.Logger) (*mongodbatlas.Client, error) {
	t := digest.NewTransport(opts.AtlasPublicKey, opts.AtlasPrivateKey)
	tc, err := t.Client()
	if err != nil {
		level.Error(logger).Log("Error connecting to Atlas:", err)
		return nil, err
	}

	client := mongodbatlas.NewClient(tc)
	return client, nil
}

func initRoutes(opts GlobalFlags, logger log.Logger, atlasClient *mongodbatlas.Client) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(opts.WebTelemetryPath, promhttp.Handler())

	// Endpoint to do scrapes.
	mux.HandleFunc(opts.WebScrapePath, func(w http.ResponseWriter, r *http.Request) {
		handler(w, r, logger, atlasClient, opts.AtlasProjectId)
	})

	rootResponse := fmt.Sprintf(`<html>
	<head>
		<title>Atlas MongoDB Prometheus Exporter</title>
	</head>
		<body>
		<p><a href="%s">Exporter Metrics</a></p>
		<p><a href="%s">API Scraped Metrics</a></p>
		</body>
	</html>`, opts.WebTelemetryPath, opts.WebScrapePath)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(rootResponse))
	})

	return mux
}

func main() {
	exitCode := 0
	defer func() { os.Exit(exitCode) }()

	promlogConfig := &promlog.Config{}
	logger := promlog.New(promlogConfig)

	var opts GlobalFlags
	_ = kong.Parse(&opts,
		kong.Name(prog),
		kong.Description("A Prometheus exporter for Atlas MongoDB API"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
		kong.Vars{
			"version": version.Version,
		})

	if opts.Version {
		fmt.Printf("Version: %s\n", version.Version)
		fmt.Printf("Commit: %s\n", version.Revision)
		fmt.Printf("Build date: %s\n", version.BuildTime)

		return
	}

	atlasClient, err := getAtlasClient(opts, logger)
	if err != nil {
		exitCode = 1

		return
	}

	initExporterMetrics()

	mux := initRoutes(opts, logger, atlasClient)

	if err := http.ListenAndServe(opts.WebListenAddress, mux); err != nil {
		level.Error(logger).Log("msg", "Error starting HTTP server", "err", err)

		exitCode = 1
	}
}
