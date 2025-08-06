package mcp

import (
	"context"
	"embed"
	"log/slog"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tjhop/prometheus-mcp-server/internal/metrics"
	"github.com/tjhop/prometheus-mcp-server/internal/version"
)

var (
	//go:embed assets/*
	assets embed.FS

	instrx    string
	toolStats toolCallStats

	metricServerReady = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: prometheus.BuildFQName(metrics.MetricNamespace, "server", "ready"),
			Help: "Info metric with a static '1' if the MCP server is ready, and '0' otherwise.",
		},
	)

	metricToolCallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:                        prometheus.BuildFQName(metrics.MetricNamespace, "tool", "call_duration_seconds"),
			Help:                        "Duration of tool calls, per tool, in seconds.",
			Buckets:                     prometheus.ExponentialBuckets(0.25, 2, 10),
			NativeHistogramBucketFactor: 1.1,
		},
		[]string{"tool_name"},
	)

	metricToolCallFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prometheus.BuildFQName(metrics.MetricNamespace, "tool", "call_failures_total"),
			Help: "Total number of failures per tool.",
		},
		[]string{"tool_name"},
	)
)

type toolCallStats struct {
	mu      sync.Mutex
	timings map[float64]time.Time
}

func init() {
	metrics.Registry.MustRegister(
		metricServerReady,
		metricToolCallDuration,
		metricToolCallFailures,
	)
}

func NewServer(logger *slog.Logger, enableTsdbAdminTools bool) *server.MCPServer {
	hooks := &server.Hooks{}

	hooks.AddAfterInitialize(func(ctx context.Context, id any, message *mcp.InitializeRequest, result *mcp.InitializeResult) {
		logger.Debug("MCP server initialized", "id", id, "mcp_message", message, "mcp_result", result)
		metricServerReady.Set(1)

		// init tool call stats
		toolStats = toolCallStats{timings: make(map[float64]time.Time)}
	})

	hooks.AddBeforeCallTool(func(ctx context.Context, id any, message *mcp.CallToolRequest) {
		method := message.Method
		params := message.Params
		args := message.GetArguments()
		logger.Debug("Before Call Tool Hook", "request_method", method, "tool_name", params.Name, "request_arguments", args)

		toolStats.mu.Lock()
		defer toolStats.mu.Unlock()
		toolStats.timings[id.(float64)] = time.Now()
	})

	hooks.AddAfterCallTool(func(ctx context.Context, id any, message *mcp.CallToolRequest, result *mcp.CallToolResult) {
		idx := id.(float64)
		name := message.Params.Name

		toolStats.mu.Lock()
		defer toolStats.mu.Unlock()
		if start, ok := toolStats.timings[idx]; ok {
			dur := time.Since(start)
			method := message.Method
			args := message.GetArguments()
			logger.Debug("After Call Tool Hook", "request_method", method, "tool_name", name, "request_arguments", args, "tool_duration", dur.String())

			// TODO: exemplars?
			metricToolCallDuration.With(prometheus.Labels{"tool_name": name}).Observe(dur.Seconds())
		}
		delete(toolStats.timings, idx)

		if result.IsError {
			// TODO: exemplars?
			metricToolCallFailures.With(prometheus.Labels{"tool_name": name}).Inc()
		}
	})

	coreInstructions, err := assets.ReadFile("assets/instructions.md")
	if err != nil {
		logger.Error("Failed to read instructions from embedded assets", "err", err)
	}
	instrx = string(coreInstructions)

	// TODO: allow users to specify additional instructions/context?

	mcpServer := server.NewMCPServer(
		"prometheus-mcp-server",
		version.Info(),
		server.WithInstructions(instrx),
		server.WithLogging(),
		server.WithRecovery(),
		server.WithHooks(hooks),
		server.WithResourceCapabilities(true, true),
	)

	// add resources
	mcpServer.AddResource(listMetricsResource, listMetricsResourceHandler)
	mcpServer.AddResource(targetsResource, targetsResourceHandler)
	mcpServer.AddResource(tsdbStatsResource, tsdbStatsResourceHandler)

	// add tools
	mcpServer.AddTool(alertmanagersTool, alertmanagersToolHandler)
	mcpServer.AddTool(buildinfoTool, buildinfoToolHandler)
	mcpServer.AddTool(configTool, configToolHandler)
	mcpServer.AddTool(exemplarQueryTool, exemplarQueryToolHandler)
	mcpServer.AddTool(flagsTool, flagsToolHandler)
	mcpServer.AddTool(labelNamesTool, labelNamesToolHandler)
	mcpServer.AddTool(labelValuesTool, labelValuesToolHandler)
	mcpServer.AddTool(listAlertsTool, listAlertsToolHandler)
	mcpServer.AddTool(metricMetadataTool, metricMetadataToolHandler)
	mcpServer.AddTool(queryTool, queryToolHandler)
	mcpServer.AddTool(rangeQueryTool, rangeQueryToolHandler)
	mcpServer.AddTool(rulesTool, rulesToolHandler)
	mcpServer.AddTool(runtimeinfoTool, runtimeinfoToolHandler)
	mcpServer.AddTool(seriesTool, seriesToolHandler)
	mcpServer.AddTool(targetsMetadataTool, targetsMetadataToolHandler)
	mcpServer.AddTool(targetsTool, targetsToolHandler)
	mcpServer.AddTool(tsdbStatsTool, tsdbStatsToolHandler)
	mcpServer.AddTool(walReplayTool, walReplayToolHandler)

	// if enabled at cli by flag, allow using the TSDB admin APIs
	if enableTsdbAdminTools {
		logger.Warn(
			"TSDB Admin APIs have been enabled!" +
				" This is dangerous, and allows for destructive operations like deleting data." +
				" It is not the fault of this MCP server if the LLM you're connected to nukes all your data.",
		)

		mcpServer.AddTool(cleanTombstonesTool, cleanTombstonesToolHandler)
		mcpServer.AddTool(deleteSeriesTool, deleteSeriesToolHandler)
		mcpServer.AddTool(snapshotTool, snapshotToolHandler)
	}

	return mcpServer
}
