package mcp

import (
	"log/slog"

	"github.com/mark3labs/mcp-go/server"

	"github.com/tjhop/prometheus-mcp-server/internal/version"
)

func NewServer(logger *slog.Logger) *server.MCPServer {
	hooks := &server.Hooks{}

	// TODO: remove/improve this hook?
	// hooks.AddBeforeCallTool(func(ctx context.Context, id any, message *mcp.CallToolRequest) {
	// logger.Debug("Before Call Tool", "id", id, "tool_request", message)
	// })

	mcpServer := server.NewMCPServer(
		"prometheus-mcp-server",
		version.Info(),
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

	return mcpServer
}
