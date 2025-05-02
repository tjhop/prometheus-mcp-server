package mcp

import (
	"context"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tjhop/prometheus-mcp-server/internal/version"
)

func NewServer(logger *slog.Logger) *server.MCPServer {
	hooks := &server.Hooks{}

	hooks.AddBeforeCallTool(func(ctx context.Context, id any, message *mcp.CallToolRequest) {
		logger.Debug("Before Call Tool", "id", id, "tool_request", message)
	})

	mcpServer := server.NewMCPServer(
		"prometheus-mcp-server",
		version.Info(),
		server.WithLogging(),
		server.WithHooks(hooks),
	)

	// add tools
	mcpServer.AddTool(execQueryTool, execQueryToolHandler)
	mcpServer.AddTool(tsdbStatsTool, tsdbStatsToolHandler)
	mcpServer.AddTool(listAlertsTool, listAlertsToolHandler)
	mcpServer.AddTool(alertmanagersTool, alertmanagersToolHandler)
	mcpServer.AddTool(flagsTool, flagsToolHandler)
	mcpServer.AddTool(buildinfoTool, buildinfoToolHandler)
	mcpServer.AddTool(runtimeinfoTool, runtimeinfoToolHandler)
	mcpServer.AddTool(rulesTool, rulesToolHandler)

	return mcpServer
}
