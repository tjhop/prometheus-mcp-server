package mcp

import (
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
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
