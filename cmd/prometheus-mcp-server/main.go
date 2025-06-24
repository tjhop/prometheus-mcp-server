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
	prometheusUrl = kingpin.Flag(
		"prometheus.url",
		"URL of the Prometheus instance to connect to",
	).Default("http://127.0.0.1:9090").String()

	flagHttpConfig = kingpin.Flag(
		"http.config",
		"Path to config file to set Prometheus HTTP client options",
	).String()

	enableTsdbAdminTools = kingpin.Flag(
		"dangerous.enable-tsdb-admin-tools",
		"Enable and allow using tools that access Prometheus' TSDB Admin API endpoints"+
			" (`snapshot`, `delete_series`, and `clean_tombstones` tools). "+
			" Docs: https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-admin-apis",
	).Default("false").Bool()
)

func main() {
	promslogConfig := &promslog.Config{}
	flag.AddFlags(kingpin.CommandLine, promslogConfig)
	kingpin.Version(version.Print(programName))
	kingpin.CommandLine.UsageWriter(os.Stdout)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promslog.New(promslogConfig)

	logger.Info("Starting "+programName, "version", version.Version, "build_date", version.BuildDate, "commit", version.Commit, "go_version", runtime.Version())

	if err := mcp.NewAPIClient(*prometheusUrl, *flagHttpConfig); err != nil {
		logger.Error("Failed to create Prometheus client for MCP server", "err", err)
	}

	mcpServer := mcp.NewServer(logger, *enableTsdbAdminTools)
	if err := server.ServeStdio(mcpServer, server.WithErrorLogger(slog.NewLogLogger(logger.Handler(), slog.LevelError))); err != nil {
		logger.Error("Prometheus MCP server failed", "err", err)
	}
}
