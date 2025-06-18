package mcp

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	mcpProm "github.com/tjhop/prometheus-mcp-server/pkg/prometheus"
)

var (
	// Tools
	queryTool = mcp.NewTool("query",
		mcp.WithDescription("Execute an instant query against the Prometheus datasource"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Query to be executed"),
		),
		mcp.WithString("timestamp",
			mcp.Description("[Optional] Timestamp for the query to be executed at. Must be either Unix timestamp or RFC3339."),
		),
	)

	rangeQueryTool = mcp.NewTool("range_query",
		mcp.WithDescription("Execute a range query against the Prometheus datasource"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Query to be executed"),
		),
		mcp.WithString("start_time",
			mcp.Description("[Optional] Start timestamp for the query to be executed at."+
				" Must be either Unix timestamp or RFC3339. Defaults to 5m ago."),
		),
		mcp.WithString("end_time",
			mcp.Description("[Optional] End timestamp for the query to be executed at."+
				" Must be either Unix timestamp or RFC3339. Defaults to current time."),
		),
		mcp.WithString("step",
			mcp.Description("[Optional] Query resolution step width in duration format or float number of seconds."+
				" It will be set automatically if unset."),
		),
	)

	exemplarQueryTool = mcp.NewTool("exemplar_query",
		mcp.WithDescription("Execute a exemplar query against the Prometheus datasource"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Query to be executed"),
		),
		mcp.WithString("start_time",
			mcp.Description("[Optional] Start timestamp for the query to be executed at."+
				" Must be either Unix timestamp or RFC3339. Defaults to 5m ago."),
		),
		mcp.WithString("end_time",
			mcp.Description("[Optional] End timestamp for the query to be executed at."+
				" Must be either Unix timestamp or RFC3339. Defaults to current time."),
		),
	)

	seriesTool = mcp.NewTool("series",
		mcp.WithDescription("Finds series by label matchers"),
		mcp.WithArray("matchers",
			mcp.Required(),
			mcp.Description("Series matchers"),
		),
		mcp.WithString("start_time",
			mcp.Description("[Optional] Start timestamp for the query to be executed at."+
				" Must be either Unix timestamp or RFC3339. Defaults to 5m ago."),
		),
		mcp.WithString("end_time",
			mcp.Description("[Optional] End timestamp for the query to be executed at."+
				" Must be either Unix timestamp or RFC3339. Defaults to current time."),
		),
	)

	labelNamesTool = mcp.NewTool("label_names",
		mcp.WithDescription("Returns the unique label names present in the block in sorted order by given time range and matchers"),
		mcp.WithArray("matchers",
			mcp.Required(),
			mcp.Description("Label matchers"),
		),
		mcp.WithString("start_time",
			mcp.Description("[Optional] Start timestamp for the query to be executed at."+
				" Must be either Unix timestamp or RFC3339. Defaults to 5m ago."),
		),
		mcp.WithString("end_time",
			mcp.Description("[Optional] End timestamp for the query to be executed at."+
				" Must be either Unix timestamp or RFC3339. Defaults to current time."),
		),
	)

	labelValuesTool = mcp.NewTool("label_values",
		mcp.WithDescription("Performs a query for the values of the given label, time range and matchers"),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("The label to query values for"),
		),
		mcp.WithArray("matchers",
			mcp.Required(),
			mcp.Description("Label matchers"),
		),
		mcp.WithString("start_time",
			mcp.Description("[Optional] Start timestamp for the query to be executed at."+
				" Must be either Unix timestamp or RFC3339. Defaults to 5m ago."),
		),
		mcp.WithString("end_time",
			mcp.Description("[Optional] End timestamp for the query to be executed at."+
				" Must be either Unix timestamp or RFC3339. Defaults to current time."),
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

	configTool = mcp.NewTool("config",
		mcp.WithDescription("Get Prometheus configuration"),
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

	targetsMetadataTool = mcp.NewTool("targets_metadata",
		mcp.WithDescription("Returns metadata about metrics currently scraped by the target "),
		mcp.WithString("match_target",
			mcp.Description("[Optional] Label selectors that match targets by their label sets. All targets are selected if left empty."),
		),
		mcp.WithString("metric",
			mcp.Description("[Optional] A metric name to retrieve metadata for. All metric metadata is retrieved if left empty."),
		),
		mcp.WithString("limit",
			mcp.Description("[Optional] Maximum number of targets to match."),
		),
	)

	metricMetadataTool = mcp.NewTool("metric_metadata",
		mcp.WithDescription("Returns metadata about metrics currently scraped by the metric name."),
		mcp.WithString("metric",
			mcp.Description("[Optional] A metric name to retrieve metadata for. All metric metadata is retrieved if left empty."),
		),
		mcp.WithString("limit",
			mcp.Description("[Optional] Maximum number of metrics to return."),
		),
	)

	walReplayTool = mcp.NewTool("wal_replay_status",
		mcp.WithDescription("Get current WAL replay status"),
	)
)

func queryToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError("query must be a string"), nil
	}

	ts := time.Now()
	argTs := request.GetString("timestamp", "")
	if argTs != "" {
		parsedTs, err := mcpProm.ParseTimestamp(argTs)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get ts from args: %#v", argTs)), nil
		}

		ts = parsedTs
	}

	data, err := queryApiCall(ctx, query, ts)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func rangeQueryToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError("query must be a string"), nil
	}

	endTs := time.Now()
	startTs := endTs.Add(DefaultLookbackDelta)
	// set default step, borrowing implementation from promtool's query range support
	// https://github.com/prometheus/prometheus/blob/df1b4da348a7c2f8c0b294ffa1f05db5f6641278/cmd/promtool/query.go#L129-L131
	resolution := math.Max(math.Floor(endTs.Sub(startTs).Seconds()/250), 1)
	// Convert seconds to nanoseconds such that time.Duration parses correctly.
	step := time.Duration(resolution) * time.Second

	argEndTime := request.GetString("end_time", "")
	if argEndTime != "" {
		parsedEndTime, err := mcpProm.ParseTimestamp(argEndTime)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to parse end_time %s from args: %s", argEndTime, err.Error())), nil
		}

		endTs = parsedEndTime
	}

	argStartTime := request.GetString("start_time", "")
	if argStartTime != "" {
		parsedStartTime, err := mcpProm.ParseTimestamp(argStartTime)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to parse start_time %s from args: %s", argStartTime, err.Error())), nil
		}

		startTs = parsedStartTime
	}

	argStep := request.GetString("step", "")
	if argStep != "" {
		parsedStep, err := time.ParseDuration(argStep)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to parse duration %s for step: %s", argStep, err.Error())), nil
		}
		step = parsedStep
	}

	data, err := rangeQueryApiCall(ctx, query, startTs, endTs, step)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func exemplarQueryToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError("query must be a string"), nil
	}

	endTs := time.Now()
	startTs := endTs.Add(DefaultLookbackDelta)

	argEndTime := request.GetString("end_time", "")
	if argEndTime != "" {
		parsedEndTime, err := mcpProm.ParseTimestamp(argEndTime)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to parse end_time %s from args: %s", argEndTime, err.Error())), nil
		}

		endTs = parsedEndTime
	}

	argStartTime := request.GetString("start_time", "")
	if argStartTime != "" {
		parsedStartTime, err := mcpProm.ParseTimestamp(argStartTime)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to parse start_time %s from args: %s", argStartTime, err.Error())), nil
		}

		startTs = parsedStartTime
	}

	data, err := exemplarQueryApiCall(ctx, query, startTs, endTs)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func seriesToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	matchers, err := request.RequireStringSlice("matchers")
	if err != nil {
		return mcp.NewToolResultError("matchers must be an array"), nil
	}

	endTs := time.Now()
	startTs := endTs.Add(DefaultLookbackDelta)

	argEndTime := request.GetString("end_time", "")
	if argEndTime != "" {
		parsedEndTime, err := mcpProm.ParseTimestamp(argEndTime)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to parse end_time %s from args: %s", argEndTime, err.Error())), nil
		}

		endTs = parsedEndTime
	}

	argStartTime := request.GetString("start_time", "")
	if argStartTime != "" {
		parsedStartTime, err := mcpProm.ParseTimestamp(argStartTime)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to parse start_time %s from args: %s", argStartTime, err.Error())), nil
		}

		startTs = parsedStartTime
	}

	data, err := seriesApiCall(ctx, matchers, startTs, endTs)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func labelNamesToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	matchers, err := request.RequireStringSlice("matchers")
	if err != nil {
		return mcp.NewToolResultError("matchers must be an array"), nil
	}

	endTs := time.Now()
	startTs := endTs.Add(DefaultLookbackDelta)

	argEndTime := request.GetString("end_time", "")
	if argEndTime != "" {
		parsedEndTime, err := mcpProm.ParseTimestamp(argEndTime)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to parse end_time %s from args: %s", argEndTime, err.Error())), nil
		}

		endTs = parsedEndTime
	}

	argStartTime := request.GetString("start_time", "")
	if argStartTime != "" {
		parsedStartTime, err := mcpProm.ParseTimestamp(argStartTime)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to parse start_time %s from args: %s", argStartTime, err.Error())), nil
		}

		startTs = parsedStartTime
	}

	data, err := labelNamesApiCall(ctx, matchers, startTs, endTs)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func labelValuesToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	label, err := request.RequireString("label")
	if err != nil {
		return mcp.NewToolResultError("label must be a string"), nil
	}

	matchers, err := request.RequireStringSlice("matchers")
	if err != nil {
		return mcp.NewToolResultError("matchers must be an array"), nil
	}

	endTs := time.Now()
	startTs := endTs.Add(DefaultLookbackDelta)

	argEndTime := request.GetString("end_time", "")
	if argEndTime != "" {
		parsedEndTime, err := mcpProm.ParseTimestamp(argEndTime)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to parse end_time %s from args: %s", argEndTime, err.Error())), nil
		}

		endTs = parsedEndTime
	}

	argStartTime := request.GetString("start_time", "")
	if argStartTime != "" {
		parsedStartTime, err := mcpProm.ParseTimestamp(argStartTime)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to parse start_time %s from args: %s", argStartTime, err.Error())), nil
		}

		startTs = parsedStartTime
	}

	data, err := labelValuesApiCall(ctx, label, matchers, startTs, endTs)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func listAlertsToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := listAlertsApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func alertmanagersToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := alertmanagersApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func tsdbStatsToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := tsdbStatsApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func flagsToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := flagsApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func buildinfoToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := buildinfoApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func configToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := configApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func runtimeinfoToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := runtimeinfoApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func rulesToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := rulesApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func targetsToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := targetsApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func targetsMetadataToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	matchTarget := request.GetString("match_target", "")
	metric := request.GetString("metric", "")
	limit := request.GetString("limit", "")

	data, err := targetsMetadataApiCall(ctx, matchTarget, metric, limit)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

// // Metadata returns metadata about metrics currently scraped by the metric name.
// Metadata(ctx context.Context, metric, limit string) (map[string][]Metadata, error)
func metricMetadataToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	metric := request.GetString("metric", "")
	limit := request.GetString("limit", "")

	data, err := metricMetadataApiCall(ctx, metric, limit)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func walReplayToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := walReplayApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}
