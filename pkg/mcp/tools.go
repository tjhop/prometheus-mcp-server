package mcp

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	mcpProm "github.com/tjhop/prometheus-mcp-server/pkg/prometheus"
)

var (
	// Tools
	execQueryTool = mcp.NewTool("execute_query",
		mcp.WithDescription("Execute an instant query against the Prometheus datasource"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Query to be executed"),
		),
		mcp.WithString("timestamp",
			mcp.Description("[Optional] Timestamp for the query to be executed at. Must be either Unix timestamp or RFC3339."),
		),
	)

	execQueryRangeTool = mcp.NewTool("execute_range_query",
		mcp.WithDescription("Execute a range query against the Prometheus datasource"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Query to be executed"),
		),
		mcp.WithString("start_time",
			mcp.Description("[Optional] Start timestamp for the query to be executed at. Must be either Unix timestamp or RFC3339. Defaults to 5m ago."),
		),
		mcp.WithString("end_time",
			mcp.Required(),
			mcp.Description("[Optional] End timestamp for the query to be executed at. Must be either Unix timestamp or RFC3339. Defaults to current time."),
		),
		mcp.WithString("step",
			mcp.Description("[Optional] Query resolution step width in duration format or float number of seconds. It will be set automatically if unset."),
		),
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

	walReplayTool = mcp.NewTool("wal_replay_status",
		mcp.WithDescription("Get current WAL replay status"),
	)
)

func execQueryToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	query, ok := arguments["query"].(string)
	if !ok {
		return nil, errors.New("query must be a string")
	}

	ts := time.Now()
	if argTs, ok := arguments["timestamp"].(string); ok {
		parsedTs, err := mcpProm.ParseTimestamp(argTs)
		if err != nil {
			return nil, fmt.Errorf("failed to get ts from args: %#v", argTs)
		}

		ts = parsedTs
	}

	data, err := executeQueryApiCall(ctx, query, ts)
	return mcp.NewToolResultText(data), err
}

func execQueryRangeToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	query, ok := arguments["query"].(string)
	if !ok {
		return nil, errors.New("query must be a string")
	}

	endTs := time.Now()
	startTs := endTs.Add(DefaultLookbackDelta)
	// set default step, borrowing implementation from promtool's query range support
	// https://github.com/prometheus/prometheus/blob/df1b4da348a7c2f8c0b294ffa1f05db5f6641278/cmd/promtool/query.go#L129-L131
	resolution := math.Max(math.Floor(endTs.Sub(startTs).Seconds()/250), 1)
	// Convert seconds to nanoseconds such that time.Duration parses correctly.
	step := time.Duration(resolution) * time.Second

	if argEndTime, ok := arguments["end_time"].(string); ok {
		parsedEndTime, err := mcpProm.ParseTimestamp(argEndTime)
		if err != nil {
			return nil, fmt.Errorf("failed to parse end_time %s from args: %w", argEndTime, err)
		}

		endTs = parsedEndTime
	}

	if argStartTime, ok := arguments["start_time"].(string); ok {
		parsedStartTime, err := mcpProm.ParseTimestamp(argStartTime)
		if err != nil {
			return nil, fmt.Errorf("failed to parse start_time %s from args: %w", argStartTime, err)
		}

		startTs = parsedStartTime
	}

	if argStep, ok := arguments["step"].(string); ok {
		parsedStep, err := time.ParseDuration(argStep)
		if err != nil {
			return nil, fmt.Errorf("failed to parse duration %s for step: %w", argStep, err)
		}
		step = parsedStep
	}

	data, err := executeQueryRangeApiCall(ctx, query, startTs, endTs, step)
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

func walReplayToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := walReplayApiCall(ctx)
	return mcp.NewToolResultText(data), err
}
