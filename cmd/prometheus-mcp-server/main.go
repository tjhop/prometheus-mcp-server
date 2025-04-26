package main

import (
	"os"
	"runtime"

	"github.com/alecthomas/kingpin/v2"
	"github.com/mark3labs/mcp-go/server"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"

	"github.com/tjhop/prometheus-mcp-server/internal/version"
	"github.com/tjhop/prometheus-mcp-server/mcp"
)

const (
	programName = "prometheus-mcp-server"
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

	mcpServer := mcp.NewServer(logger)
	if err := server.ServeStdio(mcpServer); err != nil {
		logger.Error("Prometheus MCP server failed", "err", err)
	}
}
