package mcp

import (
	"context"
	"fmt"
	"math"
	"path"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	mcpProm "github.com/tjhop/prometheus-mcp-server/pkg/prometheus"
)

var (
	// Tool Groupings.

	// CoreTools is the list of tools that are always loaded.
	CoreTools = []string{
		"docs_list",
		"docs_read",
		"docs_search",
		"query",
		"range_query",
		"metric_metadata",
		"label_names",
		"label_values",
		"series",
	}

	PrometheusTsdbAdminTools = []string{
		"clean_tombstones",
		"delete_series",
		"snapshot",
	}

	// prometheusToolset contains all the tools to interact with standard prometheus through the HTTP API.
	prometheusToolset = map[string]server.ServerTool{
		"alertmanagers":     {Tool: prometheusAlertmanagersTool, Handler: prometheusAlertmanagersToolHandler},
		"build_info":        {Tool: prometheusBuildinfoTool, Handler: prometheusBuildinfoToolHandler},
		"clean_tombstones":  {Tool: prometheusCleanTombstonesTool, Handler: prometheusCleanTombstonesToolHandler},
		"config":            {Tool: prometheusConfigTool, Handler: prometheusConfigToolHandler},
		"delete_series":     {Tool: prometheusDeleteSeriesTool, Handler: prometheusDeleteSeriesToolHandler},
		"docs_list":         {Tool: prometheusDocsListTool, Handler: prometheusDocsListToolHandler},
		"docs_read":         {Tool: prometheusDocsReadTool, Handler: prometheusDocsReadToolHandler},
		"docs_search":       {Tool: prometheusDocsSearchTool, Handler: prometheusDocsSearchToolHandler},
		"exemplar_query":    {Tool: prometheusExemplarQueryTool, Handler: prometheusExemplarQueryToolHandler},
		"flags":             {Tool: prometheusFlagsTool, Handler: prometheusFlagsToolHandler},
		"label_names":       {Tool: prometheusLabelNamesTool, Handler: prometheusLabelNamesToolHandler},
		"label_values":      {Tool: prometheusLabelValuesTool, Handler: prometheusLabelValuesToolHandler},
		"list_alerts":       {Tool: prometheusListAlertsTool, Handler: prometheusListAlertsToolHandler},
		"list_rules":        {Tool: prometheusRulesTool, Handler: prometheusRulesToolHandler},
		"metric_metadata":   {Tool: prometheusMetricMetadataTool, Handler: prometheusMetricMetadataToolHandler},
		"query":             {Tool: prometheusQueryTool, Handler: prometheusQueryToolHandler},
		"range_query":       {Tool: prometheusRangeQueryTool, Handler: prometheusRangeQueryToolHandler},
		"runtime_info":      {Tool: prometheusRuntimeinfoTool, Handler: prometheusRuntimeinfoToolHandler},
		"series":            {Tool: prometheusSeriesTool, Handler: prometheusSeriesToolHandler},
		"snapshot":          {Tool: prometheusSnapshotTool, Handler: prometheusSnapshotToolHandler},
		"targets_metadata":  {Tool: prometheusTargetsMetadataTool, Handler: prometheusTargetsMetadataToolHandler},
		"list_targets":      {Tool: prometheusTargetsTool, Handler: prometheusTargetsToolHandler},
		"tsdb_stats":        {Tool: prometheusTsdbStatsTool, Handler: prometheusTsdbStatsToolHandler},
		"wal_replay_status": {Tool: prometheusWalReplayTool, Handler: prometheusWalReplayToolHandler},
	}

	// thanosToolset contains all the tools to interact with thanos as a
	// prometheus HTTP API compatible backend.
	//
	// Currently, the only difference between thanosToolset and
	// prometheusToolset is that thanosToolset has the following tools
	// removed because they are not implemented in Thanos and return 404s:
	// [alertmanagers, config, wal_replay_status].
	thanosToolset = map[string]server.ServerTool{
		"build_info":       {Tool: prometheusBuildinfoTool, Handler: prometheusBuildinfoToolHandler},
		"clean_tombstones": {Tool: prometheusCleanTombstonesTool, Handler: prometheusCleanTombstonesToolHandler},
		"delete_series":    {Tool: prometheusDeleteSeriesTool, Handler: prometheusDeleteSeriesToolHandler},
		"docs_list":        {Tool: prometheusDocsListTool, Handler: prometheusDocsListToolHandler},
		"docs_read":        {Tool: prometheusDocsReadTool, Handler: prometheusDocsReadToolHandler},
		"docs_search":      {Tool: prometheusDocsSearchTool, Handler: prometheusDocsSearchToolHandler},
		"exemplar_query":   {Tool: prometheusExemplarQueryTool, Handler: prometheusExemplarQueryToolHandler},
		"flags":            {Tool: prometheusFlagsTool, Handler: prometheusFlagsToolHandler},
		"label_names":      {Tool: prometheusLabelNamesTool, Handler: prometheusLabelNamesToolHandler},
		"label_values":     {Tool: prometheusLabelValuesTool, Handler: prometheusLabelValuesToolHandler},
		"list_alerts":      {Tool: prometheusListAlertsTool, Handler: prometheusListAlertsToolHandler},
		"list_rules":       {Tool: prometheusRulesTool, Handler: prometheusRulesToolHandler},
		"metric_metadata":  {Tool: prometheusMetricMetadataTool, Handler: prometheusMetricMetadataToolHandler},
		"query":            {Tool: prometheusQueryTool, Handler: prometheusQueryToolHandler},
		"range_query":      {Tool: prometheusRangeQueryTool, Handler: prometheusRangeQueryToolHandler},
		"runtime_info":     {Tool: prometheusRuntimeinfoTool, Handler: prometheusRuntimeinfoToolHandler},
		"series":           {Tool: prometheusSeriesTool, Handler: prometheusSeriesToolHandler},
		"snapshot":         {Tool: prometheusSnapshotTool, Handler: prometheusSnapshotToolHandler},
		"targets_metadata": {Tool: prometheusTargetsMetadataTool, Handler: prometheusTargetsMetadataToolHandler},
		"list_targets":     {Tool: prometheusTargetsTool, Handler: prometheusTargetsToolHandler},
		"tsdb_stats":       {Tool: prometheusTsdbStatsTool, Handler: prometheusTsdbStatsToolHandler},
	}

	// PrometheusBackends is a list of directly supported Prometheus API
	// compatible backends. Backends other than prometheus itself may
	// expose a different set of tools more tailored to the backend and/or
	// change functionality of existing tools.
	PrometheusBackends = []string{
		"prometheus",
		"thanos",
	}

	// Tools Definitions.

	// Tools for Prometheus API.
	prometheusQueryTool = mcp.NewTool("query",
		mcp.WithDescription("Execute an instant query against the Prometheus datasource"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Query to be executed"),
		),
		mcp.WithString("timestamp",
			mcp.Description("[Optional] Timestamp for the query to be executed at. Must be either Unix timestamp or RFC3339."),
		),
	)

	prometheusRangeQueryTool = mcp.NewTool("range_query",
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

	prometheusExemplarQueryTool = mcp.NewTool("exemplar_query",
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

	prometheusSeriesTool = mcp.NewTool("series",
		mcp.WithDescription("Finds series by label matches"),
		mcp.WithArray("matches",
			mcp.Required(),
			mcp.Description("Series matches"),
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

	prometheusLabelNamesTool = mcp.NewTool("label_names",
		mcp.WithDescription("Returns the unique label names present in the block in sorted order by given time range and matches"),
		mcp.WithArray("matches",
			mcp.Description("[Optional] Label matches"),
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

	prometheusLabelValuesTool = mcp.NewTool("label_values",
		mcp.WithDescription("Performs a query for the values of the given label, time range and matches"),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("The label to query values for"),
		),
		mcp.WithArray("matches",
			mcp.Description("[Optional] Label matches"),
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

	prometheusTsdbStatsTool = mcp.NewTool("tsdb_stats",
		mcp.WithDescription("Get usage and cardinality statistics from the TSDB"),
	)

	prometheusListAlertsTool = mcp.NewTool("list_alerts",
		mcp.WithDescription("List all active alerts"),
	)

	prometheusAlertmanagersTool = mcp.NewTool("alertmanagers",
		mcp.WithDescription("Get overview of Prometheus Alertmanager discovery"),
	)

	prometheusFlagsTool = mcp.NewTool("flags",
		mcp.WithDescription("Get runtime flags"),
	)

	prometheusBuildinfoTool = mcp.NewTool("build_info",
		mcp.WithDescription("Get Prometheus build information"),
	)

	prometheusConfigTool = mcp.NewTool("config",
		mcp.WithDescription("Get Prometheus configuration"),
	)

	prometheusRuntimeinfoTool = mcp.NewTool("runtime_info",
		mcp.WithDescription("Get Prometheus runtime information"),
	)

	prometheusRulesTool = mcp.NewTool("list_rules",
		mcp.WithDescription("List all alerting and recording rules that are loaded"),
	)

	prometheusTargetsTool = mcp.NewTool("list_targets",
		mcp.WithDescription("Get overview of Prometheus target discovery"),
	)

	prometheusTargetsMetadataTool = mcp.NewTool("targets_metadata",
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

	prometheusMetricMetadataTool = mcp.NewTool("metric_metadata",
		mcp.WithDescription("Returns metadata about metrics currently scraped by the metric name."),
		mcp.WithString("metric",
			mcp.Description("[Optional] A metric name to retrieve metadata for. All metric metadata is retrieved if left empty."),
		),
		mcp.WithString("limit",
			mcp.Description("[Optional] Maximum number of metrics to return."),
		),
	)

	prometheusWalReplayTool = mcp.NewTool("wal_replay_status",
		mcp.WithDescription("Get current WAL replay status"),
	)

	// Tools for Prometheus TSDB admin APIs.
	prometheusCleanTombstonesTool = mcp.NewTool("clean_tombstones",
		mcp.WithDescription("Removes the deleted data from disk and cleans up the existing tombstones"),
	)

	prometheusDeleteSeriesTool = mcp.NewTool("delete_series",
		mcp.WithDescription("Deletes data for a selection of series in a time range"),
		mcp.WithArray("matches",
			mcp.Required(),
			mcp.Description("Series matches"),
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

	prometheusSnapshotTool = mcp.NewTool("snapshot",
		mcp.WithDescription("creates a snapshot of all current data into snapshots/<datetime>-<rand>"+
			" under the TSDB's data directory and returns the directory as response."),
		mcp.WithBoolean("skip_head",
			mcp.Description("[Optional] Skip data present in the head block."),
		),
	)

	// Tools for Prometheus documentation.
	prometheusDocsListTool = mcp.NewTool("docs_list",
		mcp.WithDescription("List of Official Prometheus Documentation Files."),
	)

	prometheusDocsReadTool = mcp.NewTool("docs_read",
		mcp.WithDescription("Read the named markdown file containing official Prometheus documentation from the prometheus/docs repo"),
		mcp.WithString("file",
			mcp.Required(),
			mcp.Description("The name of the documentation file to read"),
		),
	)

	prometheusDocsSearchTool = mcp.NewTool("docs_search",
		mcp.WithDescription("Search the markdown files containing official Prometheus documentation from the prometheus/docs repo"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The query to search for"),
		),
		mcp.WithNumber("limit",
			mcp.Description("The maximum number of search results to return"),
		),
	)
)

func getTextResourceContentsAsString(resourceContents []mcp.ResourceContents) string {
	var out strings.Builder

	for _, rc := range resourceContents {
		if textRC, ok := rc.(mcp.TextResourceContents); ok {
			out.WriteString(textRC.Text)
		}
	}

	return out.String()
}

func prometheusQueryToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		return mcp.NewToolResultError("failed making query api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusRangeQueryToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		return mcp.NewToolResultError("failed making range query api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusExemplarQueryToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		return mcp.NewToolResultError("failed making exemplar api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusSeriesToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	matches, err := request.RequireStringSlice("matches")
	if err != nil {
		return mcp.NewToolResultError("matches must be an array"), nil
	}

	endTs := time.Time{}
	startTs := time.Time{}

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

	data, err := seriesApiCall(ctx, matches, startTs, endTs)
	if err != nil {
		return mcp.NewToolResultError("failed making series api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusLabelNamesToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	matches := request.GetStringSlice("matches", []string{})
	endTs := time.Time{}
	startTs := time.Time{}

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

	data, err := labelNamesApiCall(ctx, matches, startTs, endTs)
	if err != nil {
		return mcp.NewToolResultError("failed making label names api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusLabelValuesToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	label, err := request.RequireString("label")
	if err != nil {
		return mcp.NewToolResultError("label must be a string"), nil
	}

	matches := request.GetStringSlice("matches", []string{})
	endTs := time.Time{}
	startTs := time.Time{}

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

	data, err := labelValuesApiCall(ctx, label, matches, startTs, endTs)
	if err != nil {
		return mcp.NewToolResultError("failed making label values api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusListAlertsToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := listAlertsApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError("failed making list alerts api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusAlertmanagersToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := alertmanagersApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError("failed making alertmanagers api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusTsdbStatsToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := tsdbStatsApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError("failed making TSDB stats api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusFlagsToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := flagsApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError("failed making flags api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusBuildinfoToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := buildinfoApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError("failed making build info api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusConfigToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := configApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError("failed making config api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusRuntimeinfoToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := runtimeinfoApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError("failed making runtime info api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusRulesToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := rulesApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError("failed making rules api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusTargetsToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := targetsApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError("failed making targets api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusTargetsMetadataToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	matchTarget := request.GetString("match_target", "")
	metric := request.GetString("metric", "")
	limit := request.GetString("limit", "")

	data, err := targetsMetadataApiCall(ctx, matchTarget, metric, limit)
	if err != nil {
		return mcp.NewToolResultError("failed making targets metadata api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

// // Metadata returns metadata about metrics currently scraped by the metric name.
func prometheusMetricMetadataToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	metric := request.GetString("metric", "")
	limit := request.GetString("limit", "")

	data, err := metricMetadataApiCall(ctx, metric, limit)
	if err != nil {
		return mcp.NewToolResultError("failed making metric metadata api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusWalReplayToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := walReplayApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError("failed making WAL replay api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusCleanTombstonesToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, err := cleanTombstonesApiCall(ctx)
	if err != nil {
		return mcp.NewToolResultError("failed making clean tombstones api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusDeleteSeriesToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	matches, err := request.RequireStringSlice("matches")
	if err != nil {
		return mcp.NewToolResultError("matches must be an array"), nil
	}

	endTs := time.Time{}
	startTs := time.Time{}

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

	data, err := deleteSeriesApiCall(ctx, matches, startTs, endTs)
	if err != nil {
		return mcp.NewToolResultError("failed making delete series api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusSnapshotToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	skipHead := request.GetBool("skip_head", false)

	data, err := snapshotApiCall(ctx, skipHead)
	if err != nil {
		return mcp.NewToolResultError("failed making snapshot api call: " + err.Error()), nil
	}
	return mcp.NewToolResultText(data), nil
}

func prometheusDocsListToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var resourceReadReq mcp.ReadResourceRequest
	resourceReadReq.Params.URI = resourcePrefix + "docs"

	// There's probably a better way to have a tool call a resource using
	// an in-process client or something similar, but this works for now.
	res, err := docsListResourceHandler(ctx, resourceReadReq)
	if err != nil {
		return mcp.NewToolResultError("failed making docs list resource call: " + err.Error()), nil
	}

	toolRes := mcp.NewToolResultText("Documentation files found")

	for _, content := range res {
		toolRes.Content = append(toolRes.Content, mcp.NewEmbeddedResource(content))
	}

	return toolRes, nil
}

func prometheusDocsReadToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	f := request.GetString("file", "")

	var resourceReadReq mcp.ReadResourceRequest
	resourceReadReq.Params.URI = resourcePrefix + path.Join("docs", f)

	args := make(map[string]any)
	args["file"] = []string{f}
	resourceReadReq.Params.Arguments = args

	// There's probably a better way to have a tool call a resource using
	// an in-process client or something similar, but this works for now.
	res, err := docsReadResourceTemplateHandler(ctx, resourceReadReq)
	if err != nil {
		return mcp.NewToolResultError("failed making docs read resource template call: " + err.Error()), nil
	}

	toolRes := mcp.NewToolResultText("File content for documentation file: " + f)

	for _, content := range res {
		toolRes.Content = append(toolRes.Content, mcp.NewEmbeddedResource(content))
	}

	return toolRes, nil
}

func prometheusDocsSearchToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	limit := request.GetInt("limit", 10)
	query := request.GetString("query", "")
	if query == "" {
		return mcp.NewToolResultError("docs search query cannot be empty"), nil
	}

	docs, err := getDocsFsFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get docs: %w", err)
	}

	matchingChunkIds, err := docs.searchDocs(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed searching for matching docs files: %w", err)
	}

	if len(matchingChunkIds) == 0 {
		return mcp.NewToolResultError("no results found for search query: " + query), nil
	}

	matchingDocsFiles := []string{}
	docsFilesSeen := make(map[string]struct{})
	for _, chunkId := range matchingChunkIds {
		parts := strings.Split(chunkId, "#")
		name := parts[0]
		docsFilesSeen[name] = struct{}{}
		matchingDocsFiles = append(matchingDocsFiles, name)
	}

	toolRes := mcp.NewToolResultText(fmt.Sprintf("Docs search found matches in the following documentation files: %q", matchingDocsFiles))

	var resourceReadReq mcp.ReadResourceRequest
	args := make(map[string]any)
	for _, file := range matchingDocsFiles {
		resourceReadReq.Params.URI = resourcePrefix + path.Join("docs", file)
		args["file"] = []string{file}
		resourceReadReq.Params.Arguments = args

		res, err := docsReadResourceTemplateHandler(ctx, resourceReadReq)
		if err != nil {
			return mcp.NewToolResultError("failed making docs read resource template call: " + err.Error()), nil
		}

		for _, content := range res {
			toolRes.Content = append(toolRes.Content, mcp.NewEmbeddedResource(content))
		}
	}

	return toolRes, nil
}
