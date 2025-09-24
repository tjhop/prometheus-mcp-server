package mcp

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
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

	metricToolCallsFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prometheus.BuildFQName(metrics.MetricNamespace, "tool", "calls_failed_total"),
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
		metricToolCallsFailed,
	)
}

// Context key for embedding Prometheus' API client into a context for use with
// tool calls. Avoids the need for global/external state to maintain the API
// client otherwise.
type apiClientKey struct{}

type apiClientLoaderMiddleware struct {
	client promv1.API
}

func newApiClientLoaderMiddleware(c promv1.API) *apiClientLoaderMiddleware {
	return &apiClientLoaderMiddleware{client: c}
}

func addApiClientToContext(ctx context.Context, c promv1.API) context.Context {
	return context.WithValue(ctx, apiClientKey{}, c)
}

func getApiClientFromContext(ctx context.Context) (promv1.API, error) {
	client, ok := ctx.Value(apiClientKey{}).(promv1.API)
	if !ok {
		return nil, errors.New("failed to get prometheus API client from context")
	}

	return client, nil
}

func (m *apiClientLoaderMiddleware) ToolMiddleware(next server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return next(addApiClientToContext(ctx, m.client), req)
	}
}

func (m *apiClientLoaderMiddleware) ResourceMiddleware(next server.ResourceHandlerFunc) server.ResourceHandlerFunc {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return next(addApiClientToContext(ctx, m.client), request)
	}
}

type docsKey struct{}
type docsLoaderMiddleware struct {
	fsys fs.FS
}

func newDocsLoaderMiddleware(fsys fs.FS) *docsLoaderMiddleware {
	docsMW := docsLoaderMiddleware{
		fsys: fsys,
	}

	return &docsMW
}

func addDocsToContext(ctx context.Context, fsys fs.FS) context.Context {
	return context.WithValue(ctx, docsKey{}, fsys)
}

func getDocsFsFromContext(ctx context.Context) (fs.FS, error) {
	docs, ok := ctx.Value(docsKey{}).(fs.FS)
	if !ok {
		return nil, errors.New("failed to get docs FS from context")
	}

	return docs, nil
}

func (m *docsLoaderMiddleware) ToolMiddleware(next server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return next(addDocsToContext(ctx, m.fsys), req)
	}
}

func (m *docsLoaderMiddleware) ResourceMiddleware(next server.ResourceHandlerFunc) server.ResourceHandlerFunc {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return next(addDocsToContext(ctx, m.fsys), request)
	}
}

func NewServer(ctx context.Context, logger *slog.Logger, apiClient promv1.API, enableTsdbAdminTools bool, docs fs.FS) *server.MCPServer {
	hooks := &server.Hooks{}

	hooks.AddAfterInitialize(func(ctx context.Context, id any, message *mcp.InitializeRequest, result *mcp.InitializeResult) {
		logger.Debug("MCP server initialized", "id", id, "mcp_message", message, "mcp_result", result)
		metricServerReady.Set(1)

		// init tool call stats
		toolStats = toolCallStats{timings: make(map[float64]time.Time)}
	})

	// TODO: @tjhop: migrate before/after tool call hooks to tool
	// middleware? would avoid need for a map to store tool call timings
	// and the mutex/synchronization around it
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
			metricToolCallsFailed.With(prometheus.Labels{"tool_name": name}).Inc()
			logger.Error("Tool call failed", "tool_name", name, "error", result)
		}
	})

	coreInstructions, err := assets.ReadFile("assets/instructions.md")
	if err != nil {
		logger.Error("Failed to read instructions from embedded assets", "err", err)
	}
	instrx = string(coreInstructions)

	// TODO: allow users to specify additional instructions/context?

	// Add middlewares.
	apiClientLoaderToolMW := newApiClientLoaderMiddleware(apiClient).ToolMiddleware
	apiClientLoaderResourceMW := newApiClientLoaderMiddleware(apiClient).ResourceMiddleware
	docsLoaderToolMW := newDocsLoaderMiddleware(docs).ToolMiddleware
	docsLoaderResourceMW := newDocsLoaderMiddleware(docs).ResourceMiddleware

	mcpServer := server.NewMCPServer(
		"prometheus-mcp-server",
		version.Info(),
		server.WithInstructions(instrx),
		server.WithLogging(),
		server.WithRecovery(),
		server.WithHooks(hooks),
		server.WithToolCapabilities(true),
		server.WithToolHandlerMiddleware(apiClientLoaderToolMW),
		server.WithToolHandlerMiddleware(docsLoaderToolMW),
		server.WithResourceHandlerMiddleware(apiClientLoaderResourceMW),
		server.WithResourceHandlerMiddleware(docsLoaderResourceMW),
		server.WithResourceCapabilities(false, true),
	)

	// add resources
	mcpServer.AddResource(listMetricsResource, listMetricsResourceHandler)
	mcpServer.AddResource(targetsResource, targetsResourceHandler)
	mcpServer.AddResource(tsdbStatsResource, tsdbStatsResourceHandler)
	mcpServer.AddResource(docsListResource, docsListResourceHandler)

	// add resource templates

	// Waiting on resource template middleware support to be added upstream
	// https://github.com/mark3labs/mcp-go/pull/582
	mcpServer.AddResourceTemplate(docsReadResourceTemplate, server.ResourceTemplateHandlerFunc(docsLoaderResourceMW(docsReadResourceTemplateHandler)))

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
	mcpServer.AddTool(docsListTool, docsListToolHandler)
	mcpServer.AddTool(docsReadTool, docsReadToolHandler)

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
