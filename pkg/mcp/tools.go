package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/alpkeskin/gotoon"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/prometheus/client_golang/prometheus"
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
		"tsdb_blocks":       {Tool: prometheusTsdbBlocksTool, Handler: prometheusTsdbBlocksToolHandler},
		"wal_replay_status": {Tool: prometheusWalReplayTool, Handler: prometheusWalReplayToolHandler},
		"healthy":           {Tool: prometheusHealthyTool, Handler: prometheusHealthyToolHandler},
		"ready":             {Tool: prometheusReadyTool, Handler: prometheusReadyToolHandler},
		"reload":            {Tool: prometheusReloadTool, Handler: prometheusReloadToolHandler},
		"quit":              {Tool: prometheusQuitTool, Handler: prometheusQuitToolHandler},
	}

	// thanosToolset contains all the tools to interact with thanos as a
	// prometheus HTTP API compatible backend.
	//
	// Tool difference from prometheusToolset:
	//  - Added
	//    - list_stores
	//  - Changed
	//  - Removed
	//    - alertmanagers
	//    - config
	//    - wal_replay_status
	//    - reload
	//    - quit
	//    - tsdb_blocks
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
		"list_stores":      {Tool: thanosStoresTool, Handler: thanosStoresToolHandler},
		"healthy":          {Tool: prometheusHealthyTool, Handler: prometheusHealthyToolHandler},
		"ready":            {Tool: prometheusReadyTool, Handler: prometheusReadyToolHandler},
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

// Utility functions that are used amongst various tool related things.

func toonOrJsonOutput(ctx context.Context, data any) (string, error) {
	toonEnabled := getToonOutputFromContext(ctx)
	if toonEnabled {
		toonEncodedData, err := gotoon.Encode(data)
		if err != nil {
			return "", fmt.Errorf("error TOON encoding data: %w", err)
		}

		return toonEncodedData, nil
	}

	jsonEncodedData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("error marshaling to JSON: %w", err)
	}

	return string(jsonEncodedData), nil
}

func getToolCallResultAsString(result *mcp.CallToolResult) string {
	var out strings.Builder
	for _, c := range result.Content {
		if text, ok := c.(mcp.TextContent); ok {
			out.WriteString(text.Text)
		}
	}

	return out.String()
}

func getTextResourceContentsAsString(resourceContents []mcp.ResourceContents) string {
	var out strings.Builder
	for _, rc := range resourceContents {
		if textRC, ok := rc.(mcp.TextResourceContents); ok {
			out.WriteString(textRC.Text)
		}
	}

	return out.String()
}

// doHttpRequest is a generalized way to use the round tripper and prometheus
// url from the api client context middleware and construct an http.Client for
// use with other endpoints, such as those from 3rd party prometheus compatible
// systems.
//
// If expectJson is true, it unmarshal's responses to a generic interface. The
// output is then either encoded as TOON or left as JSON before converting to a
// string for return.
func doHttpRequest(ctx context.Context, method string, rt http.RoundTripper, requestURL string, requestPath string, expectJson bool) (string, error) {
	fullPath, err := url.JoinPath(requestURL, requestPath)
	if err != nil {
		return "", fmt.Errorf("error constructing URL for request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullPath, nil)
	if err != nil {
		return "", fmt.Errorf("error creating HTTP request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	httpClient := http.Client{
		Transport: rt,
	}

	startTs := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making HTTP request: %w", err)
	}
	defer resp.Body.Close()
	metricApiCallDuration.With(prometheus.Labels{"target_path": requestPath}).Observe(time.Since(startTs).Seconds())

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-ok HTTP status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	var data any
	if expectJson {
		err = json.Unmarshal(body, &data)
		if err != nil {
			return "", fmt.Errorf("error unmarshaling JSON response: %w", err)
		}
	} else {
		data = string(body)
	}

	encodedData, err := toonOrJsonOutput(ctx, data)
	if err != nil {
		return "", fmt.Errorf("error encoding response from \"%s\": %w", requestPath, err)
	}

	return encodedData, nil
}
