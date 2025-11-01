package mcp

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/config"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tjhop/prometheus-mcp-server/internal/metrics"
	"github.com/tjhop/prometheus-mcp-server/internal/version"
)

var (
	//go:embed assets/*
	assets embed.FS
	instrx string

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

	metricResourceCallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:                        prometheus.BuildFQName(metrics.MetricNamespace, "resource", "call_duration_seconds"),
			Help:                        "Duration of resource calls, per resource, in seconds.",
			Buckets:                     prometheus.ExponentialBuckets(0.25, 2, 10),
			NativeHistogramBucketFactor: 1.1,
		},
		[]string{"resource_uri"},
	)

	metricResourceCallsFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prometheus.BuildFQName(metrics.MetricNamespace, "resource", "calls_failed_total"),
			Help: "Total number of failures per resource.",
		},
		[]string{"resource_uri"},
	)
)

func init() {
	metrics.Registry.MustRegister(
		metricServerReady,
		metricToolCallDuration,
		metricToolCallsFailed,
		metricResourceCallDuration,
		metricResourceCallsFailed,
	)
}

// Context key and middlewares for embedding Prometheus' API client into a
// context for use with tool/resource calls. Avoids the need for
// global/external state to maintain the API client otherwise.
type apiClientKey struct{}
type apiClientLoaderMiddleware struct {
	prometheusUrl string
	roundTripper  http.RoundTripper
	defaultClient promv1.API
	logger        *slog.Logger
}

func newApiClientLoaderMiddleware(url string, rt http.RoundTripper, log *slog.Logger) (*apiClientLoaderMiddleware, error) {
	c, err := NewAPIClient(url, rt)
	if err != nil {
		return &apiClientLoaderMiddleware{}, err
	}
	return &apiClientLoaderMiddleware{prometheusUrl: url, roundTripper: rt, defaultClient: c, logger: log}, nil
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

// If there's an Authorization header, create a new RoundTripper
// with the credentials from the header, otherwise return the
// default client.
func (m *apiClientLoaderMiddleware) getClient(header http.Header) promv1.API {
	authorization := header.Get("Authorization")
	if header != nil && authorization != "" {
		var authType, secret string
		if strings.Contains(authorization, " ") {
			authTypeCredentials := strings.Split(authorization, " ")
			if len(authTypeCredentials) != 2 {
				m.logger.Error("Invalid Authorization header, falling back to default Prometheus client", "X-Request-ID", header.Get("X-Request-ID"))
				return m.defaultClient
			}
			authType = authTypeCredentials[0]
			secret = authTypeCredentials[1]
		} else {
			m.logger.Debug("Assuming Bearer auth type for Authorization header with no type specified", "X-Request-ID", header.Get("X-Request-ID"))
			authType = "Bearer"
			secret = authorization
		}
		rt := config.NewAuthorizationCredentialsRoundTripper(authType, config.NewInlineSecret(secret), m.roundTripper)
		client, err := NewAPIClient(m.prometheusUrl, rt)
		if err != nil {
			m.logger.Error("Failed to create Prometheus client with credentials from request header", "err", err)
			return m.defaultClient
		}
		return client
	}
	return m.defaultClient
}

func (m *apiClientLoaderMiddleware) ToolMiddleware(next server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return next(addApiClientToContext(ctx, m.getClient(req.Header)), req)
	}
}

func (m *apiClientLoaderMiddleware) ResourceMiddleware(next server.ResourceHandlerFunc) server.ResourceHandlerFunc {
	return func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return next(addApiClientToContext(ctx, m.getClient(req.Header)), req)
	}
}

// Middlewares for telemetry to provide more ergonomic metrics/logging.
type telemetryMiddleware struct {
	logger *slog.Logger
}

func newTelemetryMiddleware(logger *slog.Logger) *telemetryMiddleware {
	telemetryMW := telemetryMiddleware{
		logger: logger,
	}

	return &telemetryMW
}

func (m *telemetryMiddleware) ToolMiddleware(next server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := req.Params.Name
		args := req.GetArguments()
		m.logger.Debug("Calling tool", "tool_name", name, "request_arguments", args)

		start := time.Now()
		toolResult, err := next(ctx, req)
		dur := time.Since(start)

		metricToolCallDuration.With(prometheus.Labels{"tool_name": name}).Observe(dur.Seconds())
		m.logger.Debug("Finished calling tool", "tool_name", name, "request_arguments", args, "duration", dur)
		if err != nil || toolResult.IsError {
			// TODO: exemplars?
			metricToolCallsFailed.With(prometheus.Labels{"tool_name": name}).Inc()
			m.logger.Error("Failed calling tool", "tool_name", name, "result", toolResult, "error", err)
		}

		return toolResult, err
	}
}

func (m *telemetryMiddleware) ResourceMiddleware(next server.ResourceHandlerFunc) server.ResourceHandlerFunc {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		uri := request.Params.URI
		args := request.Params.Arguments
		m.logger.Debug("Calling resource", "resource_uri", uri, "request_arguments", args)

		start := time.Now()
		resourceResult, err := next(ctx, request)
		dur := time.Since(start)

		metricResourceCallDuration.With(prometheus.Labels{"resource_uri": uri}).Observe(dur.Seconds())
		m.logger.Debug("Finished calling resource", "resource_uri", uri, "request_arguments", args, "duration", dur)
		if err != nil {
			// TODO: exemplars?
			metricResourceCallsFailed.With(prometheus.Labels{"resource_uri": uri}).Inc()
			m.logger.Error("Failed calling resource", "resource_uri", uri, "result", resourceResult, "error", err)
		}

		return resourceResult, err
	}
}

func NewServer(ctx context.Context, logger *slog.Logger,
	promUrl string,
	prometheusBackend string,
	promRt http.RoundTripper,
	enableTsdbAdminTools bool,
	enabledTools []string,
	docs fs.FS,
) *server.MCPServer {
	hooks := &server.Hooks{}

	hooks.AddAfterInitialize(func(ctx context.Context, id any, message *mcp.InitializeRequest, result *mcp.InitializeResult) {
		logger.Debug("MCP server initialized", "mcp_message", message, "mcp_result", result)
		metricServerReady.Set(1)
	})

	coreInstructions, err := assets.ReadFile("assets/instructions.md")
	if err != nil {
		logger.Error("Failed to read instructions from embedded assets", "err", err)
	}
	instrx = string(coreInstructions)

	// TODO: allow users to specify additional instructions/context?

	// Add middlewares.
	apiClientLoaderMW, err := newApiClientLoaderMiddleware(promUrl, promRt, logger)
	if err != nil {
		logger.Error("Failed to create default Prometheus client for MCP server", "err", err)
	}
	apiClientLoaderToolMW := apiClientLoaderMW.ToolMiddleware
	apiClientLoaderResourceMW := apiClientLoaderMW.ResourceMiddleware

	docsLoaderMW := newDocsLoaderMiddleware(logger, docs)
	docsLoaderToolMW := docsLoaderMW.ToolMiddleware
	docsLoaderResourceMW := docsLoaderMW.ResourceMiddleware

	telemetryMW := newTelemetryMiddleware(logger)
	telemetryToolMW := telemetryMW.ToolMiddleware
	telemetryResourceMW := telemetryMW.ResourceMiddleware

	// Actually create MCP server.
	mcpServer := server.NewMCPServer(
		"prometheus-mcp-server",
		version.Info(),
		server.WithInstructions(instrx),
		server.WithLogging(),
		server.WithRecovery(),
		server.WithResourceRecovery(),
		server.WithHooks(hooks),
		server.WithToolCapabilities(true),
		server.WithToolHandlerMiddleware(apiClientLoaderToolMW),
		server.WithToolHandlerMiddleware(docsLoaderToolMW),
		server.WithToolHandlerMiddleware(telemetryToolMW),
		server.WithResourceHandlerMiddleware(apiClientLoaderResourceMW),
		server.WithResourceHandlerMiddleware(docsLoaderResourceMW),
		server.WithResourceHandlerMiddleware(telemetryResourceMW),
		server.WithResourceCapabilities(false, true),
	)

	// Add resources.
	mcpServer.AddResource(listMetricsResource, listMetricsResourceHandler)
	mcpServer.AddResource(targetsResource, targetsResourceHandler)
	mcpServer.AddResource(tsdbStatsResource, tsdbStatsResourceHandler)
	mcpServer.AddResource(docsListResource, docsListResourceHandler)

	// Add resource templates.
	mcpServer.AddResourceTemplate(docsReadResourceTemplate, docsReadResourceTemplateHandler)

	// Assemble the set of tools to register with the server.
	var toolset []server.ServerTool

	switch {
	case len(enabledTools) == 1 && enabledTools[0] == "all":
		for _, tool := range prometheusToolset {
			toolset = append(toolset, tool)
		}
	case len(enabledTools) == 1 && enabledTools[0] == "core":
		for _, toolName := range CoreTools {
			toolset = append(toolset, prometheusToolset[toolName])
		}
	default:
		// Always include core tools.
		enabledTools = append(enabledTools, CoreTools...)
		slices.Sort(enabledTools)
		enabledTools = slices.Compact(enabledTools)

		for _, toolName := range enabledTools {
			val, ok := prometheusToolset[toolName]
			if !ok {
				logger.Warn("Failed to find tool to register", "tool_name", toolName)
				continue
			}

			// If it's a TSDB admin tool, check if we're allowed to
			// run them.
			if slices.Contains(PrometheusTsdbAdminTools, toolName) {
				if !enableTsdbAdminTools {
					logger.Error("Failed to add TSDB admin tool to toolset",
						"err", errors.New("TSDB admin tools must be enabled with `--dangerous.enable-tsdb-admin-tools` flag"),
						"tool_name", toolName,
					)
					continue
				}

				logger.Warn("Adding TSDB admin tool to toolset for registration", "tool_name", toolName)
				toolset = append(toolset, val)
				continue
			}

			logger.Debug("Adding tool to toolset for registration", "tool_name", toolName)
			toolset = append(toolset, val)
		}
	}

	// If a specific prometheus compatible backend was provided, that
	// overrides defined tools, since dynamic tool registration is specific
	// to prometheus tools and they may not be compatible with all
	// prometheus backends.
	backend := strings.ToLower(prometheusBackend)
	switch backend {
	case "": // If no backend entered, keep loaded toolset.
	case "prometheus":
		logger.Info("Setting tools based on provided prometheus backend", "backend", backend)
		var backendToolset []server.ServerTool
		for _, tool := range prometheusToolset {
			backendToolset = append(backendToolset, tool)
		}
		toolset = backendToolset
	case "thanos":
		logger.Info("Setting tools based on provided prometheus backend", "backend", backend)
		var backendToolset []server.ServerTool
		for _, tool := range thanosToolset {
			backendToolset = append(backendToolset, tool)
		}
		toolset = backendToolset
	default:
		logger.Warn("Prometheus backend does not have custom tool support, keeping the existing loaded toolset", "backend", backend, "toolset", enabledTools)
	}

	// Add tools.
	mcpServer.AddTools(toolset...)

	return mcpServer
}
