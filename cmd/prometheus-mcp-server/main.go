package main

import (
	"log/slog"
	"os"
	"runtime"

	"github.com/alecthomas/kingpin/v2"
	"github.com/mark3labs/mcp-go/server"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"

	"github.com/tjhop/prometheus-mcp-server/internal/version"
	"github.com/tjhop/prometheus-mcp-server/pkg/mcp"
)

const (
	programName = "prometheus-mcp-server"
)

var (
	flagPrometheusUrl = kingpin.Flag(
		"prometheus.url",
		"URL of the Prometheus instance to connect to",
	).Default("http://127.0.0.1:9090").String()

	flagHttpConfig = kingpin.Flag(
		"http.config",
		"Path to config file to set Prometheus HTTP client options",
	).String()

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
)

func main() {
	promslogConfig := &promslog.Config{}
	flag.AddFlags(kingpin.CommandLine, promslogConfig)
	kingpin.Version(version.Print(programName))
	kingpin.CommandLine.UsageWriter(os.Stdout)
	kingpin.HelpFlag.Short('h')
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

	logger.Info("Starting "+programName, "version", version.Version, "build_date", version.BuildDate, "commit", version.Commit, "go_version", runtime.Version())

	if err := mcp.NewAPIClient(*flagPrometheusUrl, *flagHttpConfig); err != nil {
		logger.Error("Failed to create Prometheus client for MCP server", "err", err)
	}

	mcpServer := mcp.NewServer(logger, *flagEnableTsdbAdminTools)
	if err := server.ServeStdio(mcpServer, server.WithErrorLogger(slog.NewLogLogger(logger.Handler(), slog.LevelError))); err != nil {
		logger.Error("Prometheus MCP server failed", "err", err)
	}
}
