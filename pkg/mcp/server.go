package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"

	"github.com/tjhop/prometheus-mcp-server/internal/version"
	"github.com/tjhop/prometheus-mcp-server/pkg/prometheus"
)

var (
	// package local Prometheus API client for use with mcp tools/resources/etc
	apiV1Client  v1.API
	queryTimeout = 30 * time.Second

	// Tools
	execQueryTool = mcp.NewTool("execute_query",
		mcp.WithDescription("Execute an instant query against the Prometheus datasource"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Query to be executed"),
		),
		// mcp.WithNumber("timestamp",
		// mcp.Description("Timestamp for the query to be executed at"),
		// ),
	)

	tsdbStatsTool = mcp.NewTool("tsdb_stats",
		mcp.WithDescription("Get usage and cardinality statistics from the TSDB"),
	)

	listAlertsTool = mcp.NewTool("list_alerts",
		mcp.WithDescription("List all active alerts"),
	)

	alertmanagersTool = mcp.NewTool("alertmanagers",
		mcp.WithDescription("Get overview of Prometheus Alertmanager discovery"),
	)
)

// setup pkg local APId
func NewAPIClient() error {
	client, err := prometheus.NewAPIClient()
	if err != nil {
		return fmt.Errorf("failed to create prometheus API client: %w", err)
	}

	apiV1Client = client
	return nil
}

// Handler functions
func execQueryHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	query, ok := arguments["query"].(string)
	if !ok {
		return nil, errors.New("query must be a string")
	}
	if query == "" {
		return nil, errors.New("query cannot be empty")
	}

	result, warnings, err := apiV1Client.Query(ctx, query, time.Now(), v1.WithTimeout(queryTimeout))
	if err != nil {
		return nil, fmt.Errorf("error querying Prometheus: %w", err)
	}

	if len(warnings) > 0 {
		// TODO: how do I access the logger? can I?
		fmt.Printf("Warnings: %v\n", warnings)
	}

	toolResult := ""
	if result != nil {
		toolResult = result.String()
	}

	return mcp.NewToolResultText(toolResult), nil
}

func listAlertsToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := listAlertsApiCall(ctx)
	return mcp.NewToolResultText(data), err
}

func alertmanagersToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := alertmanagersApiCall(ctx)
	return mcp.NewToolResultText(data), err
}

func tsdbStatsToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := tsdbStatsApiCall(ctx)
	return mcp.NewToolResultText(data), err
}

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
	mcpServer.AddTool(execQueryTool, execQueryHandler)
	mcpServer.AddTool(tsdbStatsTool, tsdbStatsToolHandler)
	mcpServer.AddTool(listAlertsTool, listAlertsToolHandler)
	mcpServer.AddTool(alertmanagersTool, alertmanagersToolHandler)

	return mcpServer
}
