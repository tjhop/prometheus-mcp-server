package mcp

import (
	"context"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tjhop/prometheus-mcp-server/internal/version"
)

func NewServer(logger *slog.Logger, enableTsdbAdminTools bool) *server.MCPServer {
	hooks := &server.Hooks{}

	hooks.AddBeforeCallTool(func(ctx context.Context, id any, message *mcp.CallToolRequest) {
		logger.Info("Before Call Tool Hook", "request_id", id)

		method := message.Method
		params := message.Params
		args := message.GetArguments()
		logger.Debug("Before Call Tool Hook", "request_method", method, "tool_name", params.Name, "request_arguments", args)
	})

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
