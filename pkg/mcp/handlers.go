package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tjhop/prometheus-mcp-server/internal/metrics"
	mcpProm "github.com/tjhop/prometheus-mcp-server/pkg/prometheus"
)

// Constants and shared types for handlers.
var (
	// DefaultLookbackDelta is the default time range for queries.
	DefaultLookbackDelta = -5 * time.Minute

	// Prometheus metrics for API call instrumentation.
	metricAPICallsFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prometheus.BuildFQName(metrics.MetricNamespace, "api", "calls_failed_total"),
			Help: "Total number of Prometheus API failures, per endpoint.",
		},
		[]string{"target_path"},
	)

	metricAPICallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:                        prometheus.BuildFQName(metrics.MetricNamespace, "api", "call_duration_seconds"),
			Help:                        "Duration of Prometheus API calls, per endpoint, in seconds.",
			Buckets:                     prometheus.ExponentialBuckets(0.25, 2, 10),
			NativeHistogramBucketFactor: 1.1,
		},
		[]string{"target_path"},
	)

	errTSDBAdminToolsNotEnabled = errors.New("TSDB admin tools must be enabled with `--dangerous.enable-tsdb-admin-tools` flag")
)

// Management API endpoint constants.
const (
	mgmtAPIEndpointPrefix  = "/-/"
	mgmtAPIHealthyEndpoint = mgmtAPIEndpointPrefix + "healthy"
	mgmtAPIReadyEndpoint   = mgmtAPIEndpointPrefix + "ready"
	mgmtAPIReloadEndpoint  = mgmtAPIEndpointPrefix + "reload"
	mgmtAPIQuitEndpoint    = mgmtAPIEndpointPrefix + "quit"

	// defaultRangeQueryDataPoints is the target number of data points for range
	// queries when step is not explicitly provided. The step is auto-calculated
	// to produce approximately this many data points across the query range.
	defaultRangeQueryDataPoints = 250
)

func init() {
	metrics.Registry.MustRegister(
		metricAPICallsFailed,
		metricAPICallDuration,
	)
}

// queryAPIResponse is the response structure for query API calls.
type queryAPIResponse struct {
	Result   string          `json:"result"`
	Warnings promv1.Warnings `json:"warnings"`
}

// truncateStringByLines truncates a string to the specified number of lines.
// Returns the truncated string and a boolean indicating if truncation occurred.
func truncateStringByLines(s string, limit int) (string, bool) {
	if limit <= 0 {
		// Truncation disabled.
		return s, false
	}

	endMarker := 0
	for i := range limit {
		// Start from last endMarker marker to find next newline.
		x := strings.Index(s[endMarker:], "\n")
		if x == -1 {
			// No more newlines found, we're below limit, return full string.
			return s, false
		}

		endMarker += x

		// If not the last iteration, advance endMarker marker to start
		// of next line for the next iteration.
		if i < limit-1 {
			endMarker++
		}
	}

	// Truncate string by sub-slicing to the end of the last line in the
	// limit.
	return s[:endMarker], true
}

const (
	truncationWarningTemplate = "\n\n" +
		"Warning: The result was truncated because the Prometheus MCP server was started with the flag '--prometheus.truncation-limit=%d'.\n" +
		"You may want to try optimizing your query by refining label filters or using aggregation functions to group results, where possible.\n" +
		"If needed, several tools support a 'truncation_limit'/'limit' argument that can override the global truncation limit on a per-tool-call basis.\n" +
		"This includes the ability to disable truncation on a tool call by setting the truncation limit to -1."
)

// displayTruncationWarning returns a warning message for truncated results.
func displayTruncationWarning(limit int) string {
	return fmt.Sprintf(truncationWarningTemplate, limit)
}

// parseTimeWithDefault parses a time string using ParseTimestampOrDuration.
// If the input is empty, it returns defaultVal. This consolidates the repeated
// optional time parsing pattern used across multiple handlers.
func parseTimeWithDefault(s string, defaultVal time.Time) (time.Time, error) {
	if s == "" {
		return defaultVal, nil
	}
	return mcpProm.ParseTimestampOrDuration(s)
}

// Tool handler methods for ServerContainer

// QueryHandler handles the instant query tool.
func (s *ServerContainer) QueryHandler(ctx context.Context, req *mcp.CallToolRequest, input QueryInput) (*mcp.CallToolResult, any, error) {
	if input.Query == "" {
		return newToolErrorResult("query parameter is required"), nil, nil
	}

	ts := time.Now()
	if input.Timestamp != "" {
		parsedTs, err := mcpProm.ParseTimestampOrDuration(input.Timestamp)
		if err != nil {
			return newToolErrorResult(fmt.Sprintf("failed to parse timestamp: %v", err)), nil, nil
		}
		ts = parsedTs
	}

	truncationLimit := s.GetEffectiveTruncationLimit(input.TruncationLimit)
	result, err := s.queryAPICall(ctx, input.Query, ts, truncationLimit)
	if err != nil {
		return newToolErrorResult("failed making query api call: " + err.Error()), nil, nil
	}

	return newToolTextResult(result), nil, nil
}

// RangeQueryHandler handles the range query tool.
func (s *ServerContainer) RangeQueryHandler(ctx context.Context, req *mcp.CallToolRequest, input RangeQueryInput) (*mcp.CallToolResult, any, error) {
	if input.Query == "" {
		return newToolErrorResult("query parameter is required"), nil, nil
	}

	endTs, err := parseTimeWithDefault(input.EndTime, time.Now())
	if err != nil {
		return newToolErrorResult(fmt.Sprintf("failed to parse end_time: %v", err)), nil, nil
	}

	startTs, err := parseTimeWithDefault(input.StartTime, endTs.Add(DefaultLookbackDelta))
	if err != nil {
		return newToolErrorResult(fmt.Sprintf("failed to parse start_time: %v", err)), nil, nil
	}

	// Calculate step based on actual time range (after parsing user input).
	var step time.Duration
	if input.Step != "" {
		parsedStep, err := time.ParseDuration(input.Step)
		if err != nil {
			return newToolErrorResult(fmt.Sprintf("failed to parse step: %v", err)), nil, nil
		}
		step = parsedStep
	} else {
		// Auto-calculate step to produce approximately defaultRangeQueryDataPoints data points.
		resolution := math.Max(math.Floor(endTs.Sub(startTs).Seconds()/defaultRangeQueryDataPoints), 1)
		step = time.Duration(resolution) * time.Second
	}

	truncationLimit := s.GetEffectiveTruncationLimit(input.TruncationLimit)
	result, err := s.rangeQueryAPICall(ctx, input.Query, startTs, endTs, step, truncationLimit)
	if err != nil {
		return newToolErrorResult("failed making range query api call: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// ExemplarQueryHandler handles the exemplar query tool.
func (s *ServerContainer) ExemplarQueryHandler(ctx context.Context, req *mcp.CallToolRequest, input ExemplarQueryInput) (*mcp.CallToolResult, any, error) {
	if input.Query == "" {
		return newToolErrorResult("query parameter is required"), nil, nil
	}

	endTs, err := parseTimeWithDefault(input.EndTime, time.Now())
	if err != nil {
		return newToolErrorResult(fmt.Sprintf("failed to parse end_time: %v", err)), nil, nil
	}

	startTs, err := parseTimeWithDefault(input.StartTime, endTs.Add(DefaultLookbackDelta))
	if err != nil {
		return newToolErrorResult(fmt.Sprintf("failed to parse start_time: %v", err)), nil, nil
	}

	truncationLimit := s.GetEffectiveTruncationLimit(input.TruncationLimit)
	result, err := s.exemplarQueryAPICall(ctx, input.Query, startTs, endTs, truncationLimit)
	if err != nil {
		return newToolErrorResult("failed making exemplar api call: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// SeriesHandler handles the series query tool.
func (s *ServerContainer) SeriesHandler(ctx context.Context, req *mcp.CallToolRequest, input SeriesInput) (*mcp.CallToolResult, any, error) {
	if len(input.Matches) == 0 {
		return newToolErrorResult("at least one matches parameter is required"), nil, nil
	}

	endTs, err := parseTimeWithDefault(input.EndTime, time.Time{})
	if err != nil {
		return newToolErrorResult(fmt.Sprintf("failed to parse end_time: %v", err)), nil, nil
	}

	startTs, err := parseTimeWithDefault(input.StartTime, time.Time{})
	if err != nil {
		return newToolErrorResult(fmt.Sprintf("failed to parse start_time: %v", err)), nil, nil
	}

	truncationLimit := s.GetEffectiveTruncationLimit(input.TruncationLimit)
	result, err := s.seriesAPICall(ctx, input.Matches, startTs, endTs, truncationLimit)
	if err != nil {
		return newToolErrorResult("failed making series api call: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// LabelNamesHandler handles the label names query tool.
func (s *ServerContainer) LabelNamesHandler(ctx context.Context, req *mcp.CallToolRequest, input LabelNamesInput) (*mcp.CallToolResult, any, error) {
	endTs, err := parseTimeWithDefault(input.EndTime, time.Time{})
	if err != nil {
		return newToolErrorResult(fmt.Sprintf("failed to parse end_time: %v", err)), nil, nil
	}

	startTs, err := parseTimeWithDefault(input.StartTime, time.Time{})
	if err != nil {
		return newToolErrorResult(fmt.Sprintf("failed to parse start_time: %v", err)), nil, nil
	}

	truncationLimit := s.GetEffectiveTruncationLimit(input.TruncationLimit)
	result, err := s.labelNamesAPICall(ctx, input.Matches, startTs, endTs, truncationLimit)
	if err != nil {
		return newToolErrorResult("failed making label names api call: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// LabelValuesHandler handles the label values query tool.
func (s *ServerContainer) LabelValuesHandler(ctx context.Context, req *mcp.CallToolRequest, input LabelValuesInput) (*mcp.CallToolResult, any, error) {
	if input.Label == "" {
		return newToolErrorResult("label parameter is required"), nil, nil
	}

	endTs, err := parseTimeWithDefault(input.EndTime, time.Time{})
	if err != nil {
		return newToolErrorResult(fmt.Sprintf("failed to parse end_time: %v", err)), nil, nil
	}

	startTs, err := parseTimeWithDefault(input.StartTime, time.Time{})
	if err != nil {
		return newToolErrorResult(fmt.Sprintf("failed to parse start_time: %v", err)), nil, nil
	}

	truncationLimit := s.GetEffectiveTruncationLimit(input.TruncationLimit)
	result, err := s.labelValuesAPICall(ctx, input.Label, input.Matches, startTs, endTs, truncationLimit)
	if err != nil {
		return newToolErrorResult("failed making label values api call: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// MetricMetadataHandler handles the metric metadata tool.
func (s *ServerContainer) MetricMetadataHandler(ctx context.Context, req *mcp.CallToolRequest, input MetricMetadataInput) (*mcp.CallToolResult, any, error) {
	result, err := s.metricMetadataAPICall(ctx, input.Metric, input.Limit)
	if err != nil {
		return newToolErrorResult("failed making metric metadata api call: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// TargetsMetadataHandler handles the targets metadata tool.
func (s *ServerContainer) TargetsMetadataHandler(ctx context.Context, req *mcp.CallToolRequest, input TargetsMetadataInput) (*mcp.CallToolResult, any, error) {
	result, err := s.targetsMetadataAPICall(ctx, input.MatchTarget, input.Metric, input.Limit)
	if err != nil {
		return newToolErrorResult("failed making targets metadata api call: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// AlertmanagersHandler handles the alertmanagers tool.
func (s *ServerContainer) AlertmanagersHandler(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, any, error) {
	result, err := s.alertmanagersAPICall(ctx)
	if err != nil {
		return newToolErrorResult("failed making alertmanagers api call: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// FlagsHandler handles the flags tool.
func (s *ServerContainer) FlagsHandler(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, any, error) {
	result, err := s.flagsAPICall(ctx)
	if err != nil {
		return newToolErrorResult("failed making flags api call: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// ListAlertsHandler handles the list alerts tool.
func (s *ServerContainer) ListAlertsHandler(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, any, error) {
	result, err := s.listAlertsAPICall(ctx)
	if err != nil {
		return newToolErrorResult("failed making list alerts api call: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// TsdbStatsHandler handles the TSDB stats tool.
func (s *ServerContainer) TsdbStatsHandler(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, any, error) {
	result, err := s.tsdbStatsAPICall(ctx)
	if err != nil {
		return newToolErrorResult("failed making TSDB stats api call: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// BuildInfoHandler handles the build info tool.
func (s *ServerContainer) BuildInfoHandler(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, any, error) {
	result, err := s.buildinfoAPICall(ctx)
	if err != nil {
		return newToolErrorResult("failed making build info api call: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// ConfigHandler handles the config tool.
func (s *ServerContainer) ConfigHandler(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, any, error) {
	result, err := s.configAPICall(ctx)
	if err != nil {
		return newToolErrorResult("failed making config api call: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// RuntimeInfoHandler handles the runtime info tool.
func (s *ServerContainer) RuntimeInfoHandler(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, any, error) {
	result, err := s.runtimeinfoAPICall(ctx)
	if err != nil {
		return newToolErrorResult("failed making runtime info api call: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// ListRulesHandler handles the list rules tool.
func (s *ServerContainer) ListRulesHandler(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, any, error) {
	result, err := s.rulesAPICall(ctx)
	if err != nil {
		return newToolErrorResult("failed making rules api call: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// ListTargetsHandler handles the list targets tool.
func (s *ServerContainer) ListTargetsHandler(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, any, error) {
	result, err := s.targetsAPICall(ctx)
	if err != nil {
		return newToolErrorResult("failed making targets api call: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// WalReplayHandler handles the WAL replay status tool.
func (s *ServerContainer) WalReplayHandler(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, any, error) {
	result, err := s.walReplayAPICall(ctx)
	if err != nil {
		return newToolErrorResult("failed making WAL replay api call: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// Prometheus TSDB Admin tool handlers

// CleanTombstonesHandler handles the clean tombstones admin tool.
func (s *ServerContainer) CleanTombstonesHandler(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, any, error) {
	if !s.tsdbAdminToolsEnabled {
		return newToolErrorResult("failed making clean tombstones api call: " + errTSDBAdminToolsNotEnabled.Error()), nil, nil
	}

	logger := s.GetToolLogger(req, nil)

	logger.Warn("executing TSDB admin operation: clean tombstones")

	result, err := s.cleanTombstonesAPICall(ctx)
	if err != nil {
		return newToolErrorResult("failed making clean tombstones api call: " + err.Error()), nil, nil
	}

	logger.Warn("clean tombstones completed successfully")
	return newToolTextResult(result), nil, nil
}

// DeleteSeriesHandler handles the delete series admin tool.
func (s *ServerContainer) DeleteSeriesHandler(ctx context.Context, req *mcp.CallToolRequest, input DeleteSeriesInput) (*mcp.CallToolResult, any, error) {
	if !s.tsdbAdminToolsEnabled {
		return newToolErrorResult("failed making delete series api call: " + errTSDBAdminToolsNotEnabled.Error()), nil, nil
	}

	if len(input.Matches) == 0 {
		return newToolErrorResult("at least one matches parameter is required"), nil, nil
	}

	logger := s.GetToolLogger(req, input)

	endTs, err := parseTimeWithDefault(input.EndTime, time.Time{})
	if err != nil {
		return newToolErrorResult(fmt.Sprintf("failed to parse end_time: %v", err)), nil, nil
	}

	startTs, err := parseTimeWithDefault(input.StartTime, time.Time{})
	if err != nil {
		return newToolErrorResult(fmt.Sprintf("failed to parse start_time: %v", err)), nil, nil
	}

	logger.Warn("executing TSDB admin operation: delete series")

	result, err := s.deleteSeriesAPICall(ctx, input.Matches, startTs, endTs)
	if err != nil {
		return newToolErrorResult("failed making delete series api call: " + err.Error()), nil, nil
	}

	logger.Warn("delete series completed successfully")
	return newToolTextResult(result), nil, nil
}

// SnapshotHandler handles the snapshot admin tool.
func (s *ServerContainer) SnapshotHandler(ctx context.Context, req *mcp.CallToolRequest, input SnapshotInput) (*mcp.CallToolResult, any, error) {
	if !s.tsdbAdminToolsEnabled {
		return newToolErrorResult("failed making snapshot api call: " + errTSDBAdminToolsNotEnabled.Error()), nil, nil
	}

	logger := s.GetToolLogger(req, input)

	logger.Warn("executing TSDB admin operation: snapshot")

	result, err := s.snapshotAPICall(ctx, input.SkipHead)
	if err != nil {
		return newToolErrorResult("failed making snapshot api call: " + err.Error()), nil, nil
	}

	logger.Warn("snapshot completed successfully")
	return newToolTextResult(result), nil, nil
}

// Management API handlers

// HealthyHandler handles the healthy check tool.
func (s *ServerContainer) HealthyHandler(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, any, error) {
	result, err := s.doManagementAPICall(ctx, http.MethodGet, mgmtAPIHealthyEndpoint)
	if err != nil {
		return newToolErrorResult("failed making healthy api call: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// ReadyHandler handles the ready check tool.
func (s *ServerContainer) ReadyHandler(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, any, error) {
	result, err := s.doManagementAPICall(ctx, http.MethodGet, mgmtAPIReadyEndpoint)
	if err != nil {
		return newToolErrorResult("failed making ready api call: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// ReloadHandler handles the reload config tool.
func (s *ServerContainer) ReloadHandler(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, any, error) {
	logger := s.GetToolLogger(req, nil)

	logger.Warn("triggering Prometheus configuration reload")

	result, err := s.doManagementAPICall(ctx, http.MethodPost, mgmtAPIReloadEndpoint)
	if err != nil {
		return newToolErrorResult("failed making reload api call: " + err.Error()), nil, nil
	}

	logger.Warn("reload completed successfully")
	return newToolTextResult(result), nil, nil
}

// QuitHandler handles the quit/shutdown tool.
func (s *ServerContainer) QuitHandler(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, any, error) {
	logger := s.GetToolLogger(req, nil)

	logger.Warn("triggering Prometheus shutdown")

	result, err := s.doManagementAPICall(ctx, http.MethodPost, mgmtAPIQuitEndpoint)
	if err != nil {
		return newToolErrorResult("failed making quit api call: " + err.Error()), nil, nil
	}

	logger.Warn("quit signal sent successfully - Prometheus is shutting down")
	return newToolTextResult(result), nil, nil
}

// Documentation tool handlers

// DocsListHandler handles the docs list tool.
func (s *ServerContainer) DocsListHandler(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, any, error) {
	resourceReq := &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{
			URI: resourcePrefix + "docs",
		},
	}

	resourceResult, err := s.DocsListResourceHandler(ctx, resourceReq)
	if err != nil {
		return newToolErrorResult("failed listing docs: " + err.Error()), nil, nil
	}

	return embedResourceContentsInToolResult(resourceResult, &mcp.CallToolResult{}), nil, nil
}

// DocsReadHandler handles the docs read tool.
func (s *ServerContainer) DocsReadHandler(ctx context.Context, req *mcp.CallToolRequest, input DocsReadInput) (*mcp.CallToolResult, any, error) {
	if input.File == "" {
		return newToolErrorResult("file parameter is required"), nil, nil
	}

	// Construct the resource URI with proper URL encoding
	uri := resourcePrefix + "docs/" + url.PathEscape(input.File)
	resourceReq := &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{
			URI: uri,
		},
	}

	resourceResult, err := s.DocsReadResourceHandler(ctx, resourceReq)
	if err != nil {
		return newToolErrorResult("failed reading doc file: " + err.Error()), nil, nil
	}

	return embedResourceContentsInToolResult(resourceResult, &mcp.CallToolResult{}), nil, nil
}

// DocsSearchHandler handles the docs search tool.
func (s *ServerContainer) DocsSearchHandler(ctx context.Context, req *mcp.CallToolRequest, input DocsSearchInput) (*mcp.CallToolResult, any, error) {
	logger := s.GetToolLogger(req, input)

	if input.Query == "" {
		return newToolErrorResult("query parameter is required"), nil, nil
	}

	matchingChunkIDs, err := s.SearchDocs(input.Query, input.Limit)
	if err != nil {
		return newToolErrorResult("failed searching docs: " + err.Error()), nil, nil
	}

	if len(matchingChunkIDs) == 0 {
		return newToolTextResult("No documentation found matching query: " + input.Query), nil, nil
	}

	// Extract unique file names from chunk IDs.
	matchingDocsFiles := []string{}
	docsFilesSeen := make(map[string]struct{})
	for _, chunkID := range matchingChunkIDs {
		parts := strings.Split(chunkID, "#")
		name := parts[0]
		if _, seen := docsFilesSeen[name]; !seen {
			docsFilesSeen[name] = struct{}{}
			matchingDocsFiles = append(matchingDocsFiles, name)
		}
	}

	logger.Debug("docs search completed", "matching_files", len(matchingDocsFiles))

	// Read all matching files via the resource handler and combine results.
	resourceResults := make([]*mcp.ReadResourceResult, 0, len(matchingDocsFiles))
	resourceReq := &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{}}
	for _, file := range matchingDocsFiles {
		uri := resourcePrefix + "docs/" + url.PathEscape(file)
		resourceReq.Params.URI = uri

		resourceResult, err := s.DocsReadResourceHandler(ctx, resourceReq)
		if err != nil {
			return newToolErrorResult("failed reading doc file: " + err.Error()), nil, nil
		}
		resourceResults = append(resourceResults, resourceResult)
	}

	searchSummary := fmt.Sprintf("Found %d documentation files matching query %q: %q", len(matchingDocsFiles), input.Query, matchingDocsFiles)

	combinedSearchContents := concatResourceContents(resourceResults...)
	content := make([]mcp.Content, 0, len(combinedSearchContents)+1)
	content = append(content, &mcp.TextContent{Text: searchSummary})
	for _, rc := range combinedSearchContents {
		content = append(content, &mcp.EmbeddedResource{Resource: rc})
	}

	return &mcp.CallToolResult{Content: content}, nil, nil
}

// Thanos-specific handlers

// ThanosStoresHandler handles the Thanos list stores tool.
func (s *ServerContainer) ThanosStoresHandler(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, any, error) {
	_, rt := s.GetAPIClient(ctx)

	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/stores"
	result, err := s.doHTTPRequest(ctx, http.MethodGet, rt, path, true)
	if err != nil {
		return newToolErrorResult("failed getting stores from Thanos: " + err.Error()), nil, nil
	}
	return newToolTextResult(result), nil, nil
}

// Prometheus API call methods on ServiceContainer

func (s *ServerContainer) queryAPICall(ctx context.Context, query string, ts time.Time, truncationLimit int) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/query"
	startTs := time.Now()
	result, warnings, err := client.Query(ctx, query, ts)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to execute instant query: %w", wrapErrorIfNotFound(err, path))
	}

	resultString := result.String()
	truncatedResult, truncated := truncateStringByLines(resultString, truncationLimit)
	if truncated {
		resultString = truncatedResult + displayTruncationWarning(truncationLimit)
	}
	res := queryAPIResponse{
		Result:   resultString,
		Warnings: warnings,
	}

	return s.FormatOutput(res)
}

func (s *ServerContainer) rangeQueryAPICall(ctx context.Context, query string, start, end time.Time, step time.Duration, truncationLimit int) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/query_range"
	startTs := time.Now()
	result, warnings, err := client.QueryRange(ctx, query, promv1.Range{Start: start, End: end, Step: step})
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to execute range query: %w", wrapErrorIfNotFound(err, path))
	}

	resultString := result.String()
	truncatedResult, truncated := truncateStringByLines(resultString, truncationLimit)
	if truncated {
		resultString = truncatedResult + displayTruncationWarning(truncationLimit)
	}
	res := queryAPIResponse{
		Result:   resultString,
		Warnings: warnings,
	}

	return s.FormatOutput(res)
}

func (s *ServerContainer) exemplarQueryAPICall(ctx context.Context, query string, start, end time.Time, truncationLimit int) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/query_exemplars"
	startTs := time.Now()
	res, err := client.QueryExemplars(ctx, query, start, end)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to execute exemplar query: %w", wrapErrorIfNotFound(err, path))
	}

	var resultSB strings.Builder
	for _, r := range res {
		b, err := json.Marshal(r)
		if err != nil {
			return "", fmt.Errorf("failed to marshal exemplar: %w", err)
		}
		resultSB.Write(b)
		resultSB.WriteString("\n")
	}
	resultString := resultSB.String()

	truncatedResult, truncated := truncateStringByLines(resultString, truncationLimit)
	if truncated {
		resultString = truncatedResult + displayTruncationWarning(truncationLimit)
	}
	queryResp := queryAPIResponse{
		Result:   resultString,
		Warnings: nil,
	}

	return s.FormatOutput(queryResp)
}

func (s *ServerContainer) seriesAPICall(ctx context.Context, matches []string, start, end time.Time, truncationLimit int) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/series"
	startTs := time.Now()
	result, warnings, err := client.Series(ctx, matches, start, end)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to get series: %w", wrapErrorIfNotFound(err, path))
	}

	lsets := make([]string, len(result))
	for i, lset := range result {
		lsets[i] = lset.String()
	}

	resultString := strings.Join(lsets, "\n")
	truncatedResult, truncated := truncateStringByLines(resultString, truncationLimit)
	if truncated {
		resultString = truncatedResult + displayTruncationWarning(truncationLimit)
	}
	res := queryAPIResponse{
		Result:   resultString,
		Warnings: warnings,
	}

	return s.FormatOutput(res)
}

func (s *ServerContainer) labelNamesAPICall(ctx context.Context, matches []string, start, end time.Time, truncationLimit int) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/labels"
	startTs := time.Now()
	result, warnings, err := client.LabelNames(ctx, matches, start, end)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to get label names: %w", wrapErrorIfNotFound(err, path))
	}

	resultString := strings.Join(result, "\n")
	truncatedResult, truncated := truncateStringByLines(resultString, truncationLimit)
	if truncated {
		resultString = truncatedResult + displayTruncationWarning(truncationLimit)
	}
	res := queryAPIResponse{
		Result:   resultString,
		Warnings: warnings,
	}

	return s.FormatOutput(res)
}

func (s *ServerContainer) labelValuesAPICall(ctx context.Context, label string, matches []string, start, end time.Time, truncationLimit int) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/label/:name/values"
	startTs := time.Now()
	result, warnings, err := client.LabelValues(ctx, label, matches, start, end)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to get label values: %w", wrapErrorIfNotFound(err, path))
	}

	lvals := make([]string, len(result))
	for i, lval := range result {
		lvals[i] = string(lval)
	}

	resultString := strings.Join(lvals, "\n")
	truncatedResult, truncated := truncateStringByLines(resultString, truncationLimit)
	if truncated {
		resultString = truncatedResult + displayTruncationWarning(truncationLimit)
	}
	res := queryAPIResponse{
		Result:   resultString,
		Warnings: warnings,
	}

	return s.FormatOutput(res)
}

func (s *ServerContainer) metricMetadataAPICall(ctx context.Context, metric, limit string) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/metadata"
	startTs := time.Now()

	truncationLimit := s.truncationLimit
	if limit == "" && truncationLimit != 0 {
		limit = strconv.Itoa(truncationLimit)
	}

	limitInt := 0
	if limit != "" {
		n, err := strconv.Atoi(limit)
		if err != nil {
			return "", fmt.Errorf("failed to convert limit to int: %w", err)
		}
		limitInt = n
	}

	if limitInt == 0 {
		limit = ""
	}

	mm, err := client.Metadata(ctx, metric, limit)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to get metric metadata from Prometheus: %w", wrapErrorIfNotFound(err, path))
	}

	encodedData, err := s.FormatOutput(mm)
	if err != nil {
		return "", fmt.Errorf("failed to encode metric metadata: %w", err)
	}

	if limitInt != 0 {
		encodedData += displayTruncationWarning(limitInt)
	}

	return encodedData, nil
}

func (s *ServerContainer) targetsMetadataAPICall(ctx context.Context, matchTarget, metric, limit string) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/targets/metadata"
	startTs := time.Now()

	truncationLimit := s.truncationLimit
	if limit == "" && truncationLimit != 0 {
		limit = strconv.Itoa(truncationLimit)
	}

	limitInt := 0
	if limit != "" {
		n, err := strconv.Atoi(limit)
		if err != nil {
			return "", fmt.Errorf("failed to convert limit to int: %w", err)
		}
		limitInt = n
	}

	if limitInt == 0 {
		limit = ""
	}

	tm, err := client.TargetsMetadata(ctx, matchTarget, metric, limit)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to get target metadata from Prometheus: %w", wrapErrorIfNotFound(err, path))
	}

	encodedData, err := s.FormatOutput(tm)
	if err != nil {
		return "", fmt.Errorf("failed to encode target metadata: %w", err)
	}

	if limitInt != 0 {
		encodedData += displayTruncationWarning(limitInt)
	}

	return encodedData, nil
}

func (s *ServerContainer) alertmanagersAPICall(ctx context.Context) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/alertmanagers"
	startTs := time.Now()
	ams, err := client.AlertManagers(ctx)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to get alertmanager status from Prometheus: %w", wrapErrorIfNotFound(err, path))
	}

	return s.FormatOutput(ams)
}

func (s *ServerContainer) flagsAPICall(ctx context.Context) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/status/flags"
	startTs := time.Now()
	flags, err := client.Flags(ctx)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to get runtime flags from Prometheus: %w", wrapErrorIfNotFound(err, path))
	}

	return s.FormatOutput(flags)
}

func (s *ServerContainer) listAlertsAPICall(ctx context.Context) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/alerts"
	startTs := time.Now()
	alerts, err := client.Alerts(ctx)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to get alerts from Prometheus: %w", wrapErrorIfNotFound(err, path))
	}

	return s.FormatOutput(alerts)
}

func (s *ServerContainer) tsdbStatsAPICall(ctx context.Context) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/status/tsdb"
	startTs := time.Now()
	tsdbStats, err := client.TSDB(ctx)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to get tsdb stats from Prometheus: %w", wrapErrorIfNotFound(err, path))
	}

	return s.FormatOutput(tsdbStats)
}

func (s *ServerContainer) buildinfoAPICall(ctx context.Context) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/status/buildinfo"
	startTs := time.Now()
	bi, err := client.Buildinfo(ctx)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to get build info from Prometheus: %w", wrapErrorIfNotFound(err, path))
	}

	return s.FormatOutput(bi)
}

func (s *ServerContainer) configAPICall(ctx context.Context) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/status/config"
	startTs := time.Now()
	cfg, err := client.Config(ctx)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to get configuration from Prometheus: %w", wrapErrorIfNotFound(err, path))
	}

	return s.FormatOutput(cfg)
}

func (s *ServerContainer) runtimeinfoAPICall(ctx context.Context) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/status/runtimeinfo"
	startTs := time.Now()
	ri, err := client.Runtimeinfo(ctx)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to get runtime info from Prometheus: %w", wrapErrorIfNotFound(err, path))
	}

	return s.FormatOutput(ri)
}

func (s *ServerContainer) rulesAPICall(ctx context.Context) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/rules"
	startTs := time.Now()
	rules, err := client.Rules(ctx)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to get rules from Prometheus: %w", wrapErrorIfNotFound(err, path))
	}

	return s.FormatOutput(rules)
}

func (s *ServerContainer) targetsAPICall(ctx context.Context) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/targets"
	startTs := time.Now()
	targets, err := client.Targets(ctx)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to get targets from Prometheus: %w", wrapErrorIfNotFound(err, path))
	}

	return s.FormatOutput(targets)
}

func (s *ServerContainer) walReplayAPICall(ctx context.Context) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/status/walreplay"
	startTs := time.Now()
	wal, err := client.WalReplay(ctx)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to get WAL replay status from Prometheus: %w", wrapErrorIfNotFound(err, path))
	}

	return s.FormatOutput(wal)
}

func (s *ServerContainer) cleanTombstonesAPICall(ctx context.Context) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/admin/tsdb/clean_tombstones"
	startTs := time.Now()
	err := client.CleanTombstones(ctx)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to clean tombstones from Prometheus: %w", wrapErrorIfNotFound(err, path))
	}

	return "success", nil
}

func (s *ServerContainer) deleteSeriesAPICall(ctx context.Context, matches []string, start, end time.Time) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/admin/tsdb/delete_series"
	startTs := time.Now()
	err := client.DeleteSeries(ctx, matches, start, end)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to delete series from Prometheus: %w", wrapErrorIfNotFound(err, path))
	}

	return "success", nil
}

func (s *ServerContainer) snapshotAPICall(ctx context.Context, skipHead bool) (string, error) {
	client, _ := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	path := "/api/v1/admin/tsdb/snapshot"
	startTs := time.Now()
	ss, err := client.Snapshot(ctx, skipHead)
	metricAPICallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricAPICallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("failed to create Prometheus snapshot: %w", wrapErrorIfNotFound(err, path))
	}

	return s.FormatOutput(ss)
}

func (s *ServerContainer) doManagementAPICall(ctx context.Context, method, path string) (string, error) {
	_, rt := s.GetAPIClient(ctx)
	ctx, cancel := context.WithTimeout(ctx, s.apiTimeout)
	defer cancel()

	data, err := s.doHTTPRequest(ctx, method, rt, path, false)
	if err != nil {
		return "", fmt.Errorf("failed to make Prometheus Management API call to %s: %w", path, err)
	}

	return strings.Trim(data, "\\n\""), nil
}

// doHTTPRequest makes an HTTP request using the provided round tripper.
func (s *ServerContainer) doHTTPRequest(ctx context.Context, method string, rt http.RoundTripper, requestPath string, expectJSON bool) (string, error) {
	fullPath, err := url.JoinPath(s.prometheusURL, requestPath)
	if err != nil {
		return "", fmt.Errorf("failed to construct URL for request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullPath, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	httpClient := http.Client{
		Transport: rt,
	}

	startTs := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()
	metricAPICallDuration.With(prometheus.Labels{"target_path": requestPath}).Observe(time.Since(startTs).Seconds())

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return "", &ErrEndpointNotSupported{
				Endpoint:   requestPath,
				StatusCode: resp.StatusCode,
			}
		}
		return "", fmt.Errorf("received non-ok HTTP status code: %d", resp.StatusCode)
	}

	// TODO(@tjhop): add an io.LimitReader and enforce max response body
	// size? Should it be user configurable (flag)?
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var data any
	if expectJSON {
		err = json.Unmarshal(body, &data)
		if err != nil {
			return "", fmt.Errorf("failed to unmarshal JSON response: %w", err)
		}
	} else {
		data = string(body)
	}

	return s.FormatOutput(data)
}
