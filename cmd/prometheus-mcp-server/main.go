package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/mark3labs/mcp-go/server"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/exporter-toolkit/web"
	"github.com/prometheus/exporter-toolkit/web/kingpinflag"

	"github.com/tjhop/prometheus-mcp-server/internal/metrics"
	"github.com/tjhop/prometheus-mcp-server/internal/version"
	"github.com/tjhop/prometheus-mcp-server/pkg/mcp"
)

const (
	programName = "prometheus-mcp-server"
	defaultPort = 8080
)

var (
	//go:embed external/docs/docs
	assetsDocs embed.FS
	docsFs     fs.FS

	flagMcpTools = kingpin.Flag(
		"mcp.tools",
		"List of mcp tools to load."+
			" The target `all` can be used to load all tools."+
			" The target `core` loads only the core tools: "+strings.Join(mcp.CoreTools, ",")+
			" Otherwise, it is treated as an allow-list of tools to load, in addition to the core tools."+
			" Please see project README for more information and the full list of tools.",
	).Default("all").Strings()

	flagMcpToonOutputEnabled = kingpin.Flag(
		"mcp.enable-toon-output",
		"Enable Token-Oriented Object Notation (TOON) output for tools instead of JSON",
	).Default("false").Bool()

	flagPrometheusBackend = kingpin.Flag(
		"prometheus.backend",
		"Customize the toolset for a specific Prometheus API compatible backend."+
			" Supported backends include: "+strings.Join(mcp.PrometheusBackends, ","),
	).String()

	flagPrometheusUrl = kingpin.Flag(
		"prometheus.url",
		"URL of the Prometheus instance to connect to",
	).Default("http://127.0.0.1:9090").String()

	flagPrometheusTimeout = kingpin.Flag(
		"prometheus.timeout",
		"Timeout for API calls to the Prometheus backend",
	).Default("1m").Duration()

	flagPrometheusTruncationLimit = kingpin.Flag(
		"prometheus.truncation-limit",
		"If enabled, this controls the maximum query response size in number of lines/entries provided to the LLM from the API response."+
			" LLMs can override truncation limits if needed on a per-tool-call basis via tool request arguments on supported tools."+
			" To disable truncation limits, set to 0.",
	).Default("0").Int()

	flagHttpConfig = kingpin.Flag(
		"http.config",
		"Path to config file to set Prometheus HTTP client options",
	).String()

	flagWebTelemetryPath = kingpin.Flag(
		"web.telemetry-path",
		"Path under which to expose metrics.",
	).Default("/metrics").String()

	flagWebMaxRequests = kingpin.Flag(
		"web.max-requests",
		"Maximum number of parallel scrape requests. Use 0 to disable.",
	).Default("40").Int()

	flagEnableTsdbAdminTools = kingpin.Flag(
		"dangerous.enable-tsdb-admin-tools",
		"Enable and allow using tools that access Prometheus' TSDB Admin API endpoints"+
			" (`snapshot`, `delete_series`, and `clean_tombstones` tools)."+
			" This is dangerous, and allows for destructive operations like deleting data."+
			" It is not the fault of this MCP server if the LLM you're connected to nukes all your data."+
			" Docs: https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-admin-apis",
	).Default("false").Bool()

	flagLogToFile = kingpin.Flag(
		"log.file",
		"The name of the file to log to (file rotation policies should be configured with external tools like logrotate)",
	).String()

	flagMcpTransport = kingpin.Flag(
		"mcp.transport",
		"The type of transport to use for the MCP server [`stdio`, `http`].",
	).Default("stdio").String()

	toolkitFlags = kingpinflag.AddFlags(kingpin.CommandLine, fmt.Sprintf(":%d", defaultPort))
)

func main() {
	promslogConfig := &promslog.Config{}
	flag.AddFlags(kingpin.CommandLine, promslogConfig)
	kingpin.Version(version.Print(programName))
	kingpin.CommandLine.UsageWriter(os.Stdout)
	kingpin.HelpFlag.Short('h')
	kingpin.CommandLine.DefaultEnvars()
	kingpin.Parse()

	if *flagLogToFile != "" {
		f, err := os.OpenFile(*flagLogToFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			slog.Error("Failed to open log file for writing", "file", *flagLogToFile, "err", err)
			os.Exit(1)
		}
		defer f.Close()

		promslogConfig.Writer = f
	}

	logger := promslog.New(promslogConfig)
	slog.SetDefault(logger)
	logger.Info("Starting "+programName, "version", version.Version, "build_date", version.BuildDate, "commit", version.Commit, "go_version", runtime.Version())

	// Optionally load HTTP config file to configure HTTP client for Prometheus API.
	rt, err := getRoundTripperFromConfig(*flagHttpConfig)
	if err != nil {
		logger.Error("Failed to load HTTP config file, using default HTTP round tripper", "err", err)
	}

	ctx, rootCtxCancel := context.WithCancel(context.Background())
	defer rootCtxCancel()

	// Setup static file server for embedded prometheus docs.
	docs, err := fs.Sub(assetsDocs, "external/docs/docs")
	if err != nil {
		logger.Error("Failed to create sub FS for embedded docs", "err", err)
	} else {
		docsFs = docs
	}

	mcpServer := mcp.NewServer(ctx, logger,
		*flagPrometheusUrl,
		*flagPrometheusBackend,
		*flagPrometheusTimeout,
		*flagPrometheusTruncationLimit,
		rt,
		*flagEnableTsdbAdminTools,
		*flagMcpTools,
		docsFs,
		*flagMcpToonOutputEnabled,
	)
	srv := setupServer(logger)

	var g run.Group
	{
		// termination and cleanup
		term := make(chan os.Signal, 1)
		signal.Notify(term, os.Interrupt, syscall.SIGTERM)
		cancel := make(chan struct{})
		g.Add(
			func() error {
				select {
				case sig := <-term:
					logger.Warn("caught signal, exiting gracefully.", "signal", sig.String())
				case <-cancel:
				}

				return nil
			},
			func(err error) {
				close(cancel)
				rootCtxCancel()
			},
		)
	}
	{
		// web server for metrics, pprof, and HTTP if transport is configured
		cancel := make(chan struct{})

		g.Add(
			func() error {
				err := web.ListenAndServe(srv, toolkitFlags, logger)
				if !errors.Is(err, http.ErrServerClosed) {
					logger.Error("webserver failed", "err", err)
					return err
				}

				<-cancel

				return nil
			},
			func(error) {
				if err := srv.Shutdown(ctx); err != nil {
					// Error from closing listeners, or context timeout:
					logger.Error("failed to close listeners/context timeout", "err", err)
				}
				close(cancel)
				rootCtxCancel()
			},
		)
	}
	{
		// MCP transport setup and server
		cancel := make(chan struct{})

		g.Add(
			func() error {
				switch *flagMcpTransport {
				case "stdio":
					logger.Debug("starting MCP server", "transport", "stdio")

					stdioMcpSrv := server.NewStdioServer(mcpServer)
					server.WithErrorLogger(slog.NewLogLogger(logger.Handler(), slog.LevelError))
					if err := stdioMcpSrv.Listen(ctx, os.Stdin, os.Stdout); err != nil {
						return fmt.Errorf("MCP server failed: %w", err)
					}

				case "http":
					logger.Debug("starting MCP server", "transport", "http")

					httpMcpServer := server.NewStreamableHTTPServer(mcpServer)
					http.Handle("/mcp", httpMcpServer)
					<-cancel
				default:
					return fmt.Errorf("unsupported transport type: %s", *flagMcpTransport)
				}

				return nil
			},
			func(err error) {
				close(cancel)
				rootCtxCancel()
			},
		)
	}

	if err := g.Run(); err != nil {
		logger.Error("Failed to run daemon goroutines", "err", err)
		rootCtxCancel()

		// Gocritic complains here because of the deferred call to
		// cancel the root context above not getting called when
		// os.Exit runs immediately. Valid, but we already call the
		// cancel func in each run group's interrupt func (it's safe to
		// call a context cancellation func multiple times, it's a noop
		// after the first call). We also explicitly call it prior to
		// exit here. I can refactor main to call a secondary function
		// that returns an error to avoid conflicting with the
		// context/cancellation lifecycle, but that becomes trickier
		// with the main func because there's also a deferred call to
		// close the log file if using `--log.file=$file`, and it
		// doesn't feel worth the effort. We already know the context
		// is cancelled by this point, just ask gocritic to be quiet.
		//
		//nolint:gocritic
		os.Exit(1)
	}
	logger.Info("See you next time!")
}

func setupServer(logger *slog.Logger) *http.Server {
	server := &http.Server{
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	metricsHandler := promhttp.HandlerFor(
		prometheus.Gatherers{metrics.Registry},
		promhttp.HandlerOpts{
			ErrorLog:            slog.NewLogLogger(logger.Handler(), slog.LevelError),
			ErrorHandling:       promhttp.ContinueOnError,
			MaxRequestsInFlight: *flagWebMaxRequests,
			Registry:            metrics.Registry,
		},
	)
	metricsHandler = promhttp.InstrumentMetricHandler(
		metrics.Registry, metricsHandler,
	)
	http.Handle("/metrics", metricsHandler)

	landingPageLinks := []web.LandingLinks{
		{
			Address: *flagWebTelemetryPath,
			Text:    "Metrics",
		},
	}

	if *flagMcpTransport == "http" {
		landingPageLinks = append(landingPageLinks,
			web.LandingLinks{
				Address: "/mcp",
				Text:    "Prometheus MCP Server",
			},
		)
	}

	if docsFs != nil {
		http.Handle("/docs/", http.StripPrefix("/docs/", http.FileServer(http.FS(docsFs))))
		landingPageLinks = append(landingPageLinks,
			web.LandingLinks{
				Address: "/docs/",
				Text:    "Prometheus Documentation",
			},
		)
	}

	if *flagWebTelemetryPath != "/" {
		landingConfig := web.LandingConfig{
			Name:        "Prometheus MCP Server",
			Description: "MCP Server to interact with Prometheus",
			Version:     version.Info(),
			Links:       landingPageLinks,
		}
		landingPage, err := web.NewLandingPage(landingConfig)
		if err != nil {
			logger.Error("Failed to create landing page", "err", err)
			os.Exit(1)
		}
		http.Handle("/", landingPage)
	}

	return server
}

func getRoundTripperFromConfig(httpConfig string) (http.RoundTripper, error) {
	httpClient := http.DefaultClient
	if httpConfig != "" {
		httpCfg, _, err := config_util.LoadHTTPConfigFile(httpConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load HTTP configuration file %s: %w", httpConfig, err)
		}

		if err = httpCfg.Validate(); err != nil {
			return nil, fmt.Errorf("failed to validate HTTP configuration file %s: %w", httpConfig, err)
		}

		httpClient, err = config_util.NewClientFromConfig(*httpCfg, programName)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP client from configuration file %s: %w", httpConfig, err)
		}
	}

	rt := http.DefaultTransport
	if httpClient.Transport != nil {
		rt = httpClient.Transport
	}

	return rt, nil
}
