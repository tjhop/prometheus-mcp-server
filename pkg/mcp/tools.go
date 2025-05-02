package mcp

import (
	"context"
	"errors"

	"github.com/mark3labs/mcp-go/mcp"
)

var (
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

	flagsTool = mcp.NewTool("flags",
		mcp.WithDescription("Get runtime flags"),
	)

	buildinfoTool = mcp.NewTool("build_info",
		mcp.WithDescription("Get Prometheus build information"),
	)

	runtimeinfoTool = mcp.NewTool("runtime_info",
		mcp.WithDescription("Get Prometheus runtime information"),
	)

	rulesTool = mcp.NewTool("list_rules",
		mcp.WithDescription("List all alerting and recording rules that are loaded"),
	)

	targetsTool = mcp.NewTool("list_targets",
		mcp.WithDescription("Get overview of Prometheus target discovery"),
	)
)

func execQueryToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	query, ok := arguments["query"].(string)
	if !ok {
		return nil, errors.New("query must be a string")
	}

	data, err := executeQueryApiCall(ctx, query)
	return mcp.NewToolResultText(data), err
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

func flagsToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := flagsApiCall(ctx)
	return mcp.NewToolResultText(data), err
}

func buildinfoToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := buildinfoApiCall(ctx)
	return mcp.NewToolResultText(data), err
}

func runtimeinfoToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := runtimeinfoApiCall(ctx)
	return mcp.NewToolResultText(data), err
}

func rulesToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := rulesApiCall(ctx)
	return mcp.NewToolResultText(data), err
}

func targetsToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := targetsApiCall(ctx)
	return mcp.NewToolResultText(data), err
}
