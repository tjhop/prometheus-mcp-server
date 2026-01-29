package mcp

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/client_golang/prometheus"
)

// MCP method names. Needed because the go-sdk does not export them.
// https://github.com/modelcontextprotocol/go-sdk/blob/13488f7da1ed8eda47413df3420ab64026696798/mcp/protocol.go#L1330-L1357
const (
	methodInitialize    = "initialize"
	methodToolsCall     = "tools/call"
	methodResourcesRead = "resources/read"
)

// telemetryMiddleware creates an MCP middleware that instruments MCP method
// calls with metrics and logging.
func telemetryMiddleware(logger *slog.Logger) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			switch method {
			case methodInitialize:
				return telemetryHandleInitialize(ctx, method, req, next, logger)
			case methodToolsCall:
				return telemetryHandleToolCall(ctx, method, req, next, logger)
			case methodResourcesRead:
				return telemetryHandleResourceRead(ctx, method, req, next, logger)
			}

			// If no supported method matches, run handler directly
			// without additional instrumentation.
			return next(ctx, method, req)
		}
	}
}

// telemetryHandleInitialize handles the MCP initialization, logging client
// info and setting the server ready metric after successful initialization.
func telemetryHandleInitialize(ctx context.Context, method string, req mcp.Request, next mcp.MethodHandler, logger *slog.Logger) (mcp.Result, error) {
	params, ok := req.GetParams().(*mcp.InitializeParams)
	if !ok {
		// Can't extract init params, pass through without instrumentation.
		logger.Warn("Failed to extract initialize params for telemetry", "method", method)
		return next(ctx, method, req)
	}

	result, err := next(ctx, method, req)
	if err != nil {
		logger.Error("MCP initialization failed", "error", err)
		return result, err
	}

	// Extract client info for logging.
	clientName := ""
	clientVersion := ""
	if params.ClientInfo != nil {
		clientName = params.ClientInfo.Name
		clientVersion = params.ClientInfo.Version
	}

	// Extract server info from result for logging.
	serverName := ""
	serverVersion := ""
	if initResult, ok := result.(*mcp.InitializeResult); ok && initResult.ServerInfo != nil {
		serverName = initResult.ServerInfo.Name
		serverVersion = initResult.ServerInfo.Version
	}

	logger.Debug("MCP server initialized",
		"client_name", clientName,
		"client_version", clientVersion,
		"protocol_version", params.ProtocolVersion,
		"server_name", serverName,
		"server_version", serverVersion,
	)

	metricServerReady.Set(1)

	return result, nil
}

// telemetryHandleToolCall instruments a tools/call request with metrics and logging.
func telemetryHandleToolCall(ctx context.Context, method string, req mcp.Request, next mcp.MethodHandler, logger *slog.Logger) (mcp.Result, error) {
	params, ok := req.GetParams().(*mcp.CallToolParamsRaw)
	if !ok {
		// Can't extract tool info, pass through without instrumentation.
		logger.Warn("Failed to extract tool params for telemetry", "method", method)
		return next(ctx, method, req)
	}

	toolName := params.Name
	args := params.Arguments
	logger = logger.With("tool_name", toolName, "request_arguments", args)

	logger.Debug("Calling tool")
	startTime := time.Now()
	result, err := next(ctx, method, req)
	duration := time.Since(startTime)

	metricToolCallDuration.With(prometheus.Labels{"tool_name": toolName}).Observe(duration.Seconds())
	logger.Debug("Finished calling tool", "duration", duration)
	toolResult, ok := result.(*mcp.CallToolResult)
	if !ok {
		metricToolCallsFailed.With(prometheus.Labels{"tool_name": toolName}).Inc()
		logger.Error("Failed to convert result to call tool result")
		return result, err
	}
	if err != nil || toolResult.IsError {
		metricToolCallsFailed.With(prometheus.Labels{"tool_name": toolName}).Inc()
		logger.Error("Failed calling tool", "result", result, "error", err)
	}

	return result, err
}

// telemetryHandleResourceRead instruments a resources/read request with metrics and logging.
func telemetryHandleResourceRead(ctx context.Context, method string, req mcp.Request, next mcp.MethodHandler, logger *slog.Logger) (mcp.Result, error) {
	params, ok := req.GetParams().(*mcp.ReadResourceParams)
	if !ok {
		// Can't extract resource info, pass through without instrumentation.
		logger.Warn("Failed to extract resource params for telemetry", "method", method)
		return next(ctx, method, req)
	}

	uri := params.URI
	logger = logger.With("resource_uri", uri)

	logger.Debug("Calling resource")

	startTime := time.Now()
	result, err := next(ctx, method, req)
	duration := time.Since(startTime)

	metricResourceCallDuration.With(prometheus.Labels{"resource_uri": uri}).Observe(duration.Seconds())
	logger.Debug("Finished calling resource", "duration", duration)

	if err != nil {
		metricResourceCallsFailed.With(prometheus.Labels{"resource_uri": uri}).Inc()
		logger.Error("Failed calling resource", "result", result, "error", err)
	}

	return result, err
}
