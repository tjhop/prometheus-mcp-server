package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tjhop/prometheus-mcp-server/internal/metrics"
	mcpProm "github.com/tjhop/prometheus-mcp-server/pkg/prometheus"
)

var (
	DefaultLookbackDelta = -5 * time.Minute

	apiTimeout time.Duration

	metricApiCallsFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prometheus.BuildFQName(metrics.MetricNamespace, "api", "calls_failed_total"),
			Help: "Total number of Prometheus API failures, per endpoint.",
		},
		[]string{"target_path"},
	)

	metricApiCallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:                        prometheus.BuildFQName(metrics.MetricNamespace, "api", "call_duration_seconds"),
			Help:                        "Duration of Prometheus API calls, per endpoint, in seconds.",
			Buckets:                     prometheus.ExponentialBuckets(0.25, 2, 10),
			NativeHistogramBucketFactor: 1.1,
		},
		[]string{"target_path"},
	)
)

func init() {
	metrics.Registry.MustRegister(
		metricApiCallsFailed,
		metricApiCallDuration,
	)
}

// NewAPIClient creates a new prometheus v1 API client for use by the MCP server.
func NewAPIClient(prometheusUrl string, rt http.RoundTripper) (promv1.API, error) {
	client, err := mcpProm.NewAPIClient(prometheusUrl, rt)
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus API client: %w", err)
	}

	return client, nil
}

func alertmanagersApiCall(ctx context.Context) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/alertmanagers"
	startTs := time.Now()
	ams, err := client.AlertManagers(ctx)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error getting alertmanager status from Prometheus: %w", err)
	}

	encodedData, err := toonOrJsonOutput(ctx, ams)
	if err != nil {
		return "", fmt.Errorf("error encoding alertmanager status: %w", err)
	}

	return encodedData, nil
}

func flagsApiCall(ctx context.Context) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/status/flags"
	startTs := time.Now()
	flags, err := client.Flags(ctx)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error getting runtime flags from Prometheus: %w", err)
	}

	encodedData, err := toonOrJsonOutput(ctx, flags)
	if err != nil {
		return "", fmt.Errorf("error encoding runtime flags: %w", err)
	}

	return encodedData, nil
}

func listAlertsApiCall(ctx context.Context) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/alerts"
	startTs := time.Now()
	alerts, err := client.Alerts(ctx)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error getting alerts from Prometheus: %w", err)
	}

	encodedData, err := toonOrJsonOutput(ctx, alerts)
	if err != nil {
		return "", fmt.Errorf("error encoding alerts: %w", err)
	}

	return encodedData, nil
}

func tsdbStatsApiCall(ctx context.Context) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/status/tsdb"
	startTs := time.Now()
	tsdbStats, err := client.TSDB(ctx)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error getting tsdb stats from Prometheus: %w", err)
	}

	encodedData, err := toonOrJsonOutput(ctx, tsdbStats)
	if err != nil {
		return "", fmt.Errorf("error encoding tsdb stats: %w", err)
	}

	return encodedData, nil
}

type queryApiResponse struct {
	Result   string          `json:"result"`
	Warnings promv1.Warnings `json:"warnings"`
}

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
		"This includes the ability to disable truncation on a tool call by setting the truncation limit to 0."
)

func displayTruncationWarning(limit int) string {
	return fmt.Sprintf(truncationWarningTemplate, limit)
}

func queryApiCall(ctx context.Context, query string, ts time.Time) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/query"
	startTs := time.Now()
	result, warnings, err := client.Query(ctx, query, ts)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error executing instant query: %w", err)
	}

	resultString := result.String()
	truncationLimit := getTruncationFromContext(ctx)
	truncatedResult, truncated := truncateStringByLines(resultString, truncationLimit)
	if truncated {
		resultString = truncatedResult + displayTruncationWarning(truncationLimit)
	}
	res := queryApiResponse{
		Result:   resultString,
		Warnings: warnings,
	}

	encodedData, err := toonOrJsonOutput(ctx, res)
	if err != nil {
		return "", fmt.Errorf("error encoding query response: %w", err)
	}

	return encodedData, nil
}

func rangeQueryApiCall(ctx context.Context, query string, start, end time.Time, step time.Duration) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/query_range"
	startTs := time.Now()
	result, warnings, err := client.QueryRange(ctx, query, promv1.Range{Start: start, End: end, Step: step})
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error executing range query: %w", err)
	}

	resultString := result.String()
	truncationLimit := getTruncationFromContext(ctx)
	truncatedResult, truncated := truncateStringByLines(resultString, truncationLimit)
	if truncated {
		resultString = truncatedResult + displayTruncationWarning(truncationLimit)
	}
	res := queryApiResponse{
		Result:   resultString,
		Warnings: warnings,
	}

	encodedData, err := toonOrJsonOutput(ctx, res)
	if err != nil {
		return "", fmt.Errorf("error encoding query response: %w", err)
	}

	return encodedData, nil
}

func exemplarQueryApiCall(ctx context.Context, query string, start, end time.Time) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/query_exemplars"
	startTs := time.Now()
	res, err := client.QueryExemplars(ctx, query, start, end)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error executing exemplar query: %w", err)
	}

	var resultSB strings.Builder
	for _, r := range res {
		b, err := json.Marshal(r)
		if err != nil {
			return "", fmt.Errorf("error marshaling exemplar: %w", err)
		}

		resultSB.Write(b)
		resultSB.WriteString("\n")
	}
	resultString := resultSB.String()

	truncationLimit := getTruncationFromContext(ctx)
	truncatedResult, truncated := truncateStringByLines(resultString, truncationLimit)
	if truncated {
		resultString = truncatedResult + displayTruncationWarning(truncationLimit)
	}
	queryResp := queryApiResponse{
		Result:   resultString,
		Warnings: nil,
	}

	encodedData, err := toonOrJsonOutput(ctx, queryResp)
	if err != nil {
		return "", fmt.Errorf("error encoding query response: %w", err)
	}

	return encodedData, nil
}

func seriesApiCall(ctx context.Context, matches []string, start, end time.Time) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/series"
	startTs := time.Now()
	result, warnings, err := client.Series(ctx, matches, start, end)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error getting series: %w", err)
	}

	// convert labelsets to their stringified form
	lsets := make([]string, len(result))
	for i, lset := range result {
		lsets[i] = lset.String()
	}

	resultString := strings.Join(lsets, "\n")
	truncationLimit := getTruncationFromContext(ctx)
	truncatedResult, truncated := truncateStringByLines(resultString, truncationLimit)
	if truncated {
		resultString = truncatedResult + displayTruncationWarning(truncationLimit)
	}
	res := queryApiResponse{
		Result:   resultString,
		Warnings: warnings,
	}

	encodedData, err := toonOrJsonOutput(ctx, res)
	if err != nil {
		return "", fmt.Errorf("error encoding series response: %w", err)
	}

	return encodedData, nil
}

func labelNamesApiCall(ctx context.Context, matches []string, start, end time.Time) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/labels"
	startTs := time.Now()
	result, warnings, err := client.LabelNames(ctx, matches, start, end)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error getting label names: %w", err)
	}

	resultString := strings.Join(result, "\n")
	truncationLimit := getTruncationFromContext(ctx)
	truncatedResult, truncated := truncateStringByLines(resultString, truncationLimit)
	if truncated {
		resultString = truncatedResult + displayTruncationWarning(truncationLimit)
	}
	res := queryApiResponse{
		Result:   resultString,
		Warnings: warnings,
	}

	encodedData, err := toonOrJsonOutput(ctx, res)
	if err != nil {
		return "", fmt.Errorf("error encoding label names response: %w", err)
	}

	return encodedData, nil
}

func labelValuesApiCall(ctx context.Context, label string, matches []string, start, end time.Time) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/label/:name/values"
	startTs := time.Now()
	result, warnings, err := client.LabelValues(ctx, label, matches, start, end)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error getting label values: %w", err)
	}

	lvals := make([]string, len(result))
	for i, lval := range result {
		lvals[i] = string(lval)
	}

	resultString := strings.Join(lvals, "\n")
	truncationLimit := getTruncationFromContext(ctx)
	truncatedResult, truncated := truncateStringByLines(resultString, truncationLimit)
	if truncated {
		resultString = truncatedResult + displayTruncationWarning(truncationLimit)
	}
	res := queryApiResponse{
		Result:   resultString,
		Warnings: warnings,
	}

	encodedData, err := toonOrJsonOutput(ctx, res)
	if err != nil {
		return "", fmt.Errorf("error encoding label values response: %w", err)
	}

	return encodedData, nil
}

func buildinfoApiCall(ctx context.Context) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/status/buildinfo"
	startTs := time.Now()
	bi, err := client.Buildinfo(ctx)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error getting build info from Prometheus: %w", err)
	}

	encodedData, err := toonOrJsonOutput(ctx, bi)
	if err != nil {
		return "", fmt.Errorf("error encoding build info: %w", err)
	}

	return encodedData, nil
}

func configApiCall(ctx context.Context) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/status/config"
	startTs := time.Now()
	cfg, err := client.Config(ctx)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error getting configuration from Prometheus: %w", err)
	}

	encodedData, err := toonOrJsonOutput(ctx, cfg)
	if err != nil {
		return "", fmt.Errorf("error encoding configuration: %w", err)
	}

	return encodedData, nil
}

func runtimeinfoApiCall(ctx context.Context) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/status/runtimeinfo"
	startTs := time.Now()
	ri, err := client.Runtimeinfo(ctx)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error getting runtime info from Prometheus: %w", err)
	}

	encodedData, err := toonOrJsonOutput(ctx, ri)
	if err != nil {
		return "", fmt.Errorf("error encoding runtime info: %w", err)
	}

	return encodedData, nil
}

func rulesApiCall(ctx context.Context) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/rules"
	startTs := time.Now()
	rules, err := client.Rules(ctx)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error getting rules from Prometheus: %w", err)
	}

	// TODO: @tjhop: Impose truncation limit on rules? We get back a slice
	// of rule groups from the API, and each group has a []any containing
	// it's recording/alerting rules. Where/how is most appropriate to
	// truncate?
	encodedData, err := toonOrJsonOutput(ctx, rules)
	if err != nil {
		return "", fmt.Errorf("error encoding rules: %w", err)
	}

	return encodedData, nil
}

func targetsApiCall(ctx context.Context) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/targets"
	startTs := time.Now()
	targets, err := client.Targets(ctx)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error getting targets from Prometheus: %w", err)
	}

	encodedData, err := toonOrJsonOutput(ctx, targets)
	if err != nil {
		return "", fmt.Errorf("error encoding targets response: %w", err)
	}

	return encodedData, nil
}

func targetsMetadataApiCall(ctx context.Context, matchTarget, metric, limit string) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/targets/metadata"
	startTs := time.Now()

	truncationLimit := getTruncationFromContext(ctx)
	if limit == "" && truncationLimit != 0 {
		limit = strconv.Itoa(truncationLimit)
	}

	limitInt := 0
	if limit != "" {
		n, err := strconv.Atoi(limit)
		if err != nil {
			return "", fmt.Errorf("error converting limit to int: %w", err)
		}
		limitInt = n
	}

	if limitInt == 0 {
		limit = ""
	}

	tm, err := client.TargetsMetadata(ctx, matchTarget, metric, limit)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error getting target metadata from Prometheus: %w", err)
	}

	encodedData, err := toonOrJsonOutput(ctx, tm)
	if err != nil {
		return "", fmt.Errorf("error encoding target metadata: %w", err)
	}

	if limitInt != 0 {
		encodedData += displayTruncationWarning(limitInt)
	}

	return encodedData, nil
}

func metricMetadataApiCall(ctx context.Context, metric, limit string) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/metadata"
	startTs := time.Now()

	truncationLimit := getTruncationFromContext(ctx)
	if limit == "" && truncationLimit != 0 {
		limit = strconv.Itoa(truncationLimit)
	}

	limitInt := 0
	if limit != "" {
		n, err := strconv.Atoi(limit)
		if err != nil {
			return "", fmt.Errorf("error converting limit to int: %w", err)
		}
		limitInt = n
	}

	if limitInt == 0 {
		limit = ""
	}

	mm, err := client.Metadata(ctx, metric, limit)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error getting metric metadata from Prometheus: %w", err)
	}

	encodedData, err := toonOrJsonOutput(ctx, mm)
	if err != nil {
		return "", fmt.Errorf("error encoding metric metadata: %w", err)
	}

	if limitInt != 0 {
		encodedData += displayTruncationWarning(limitInt)
	}

	return encodedData, nil
}

func walReplayApiCall(ctx context.Context) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/status/walreplay"
	startTs := time.Now()
	wal, err := client.WalReplay(ctx)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error getting WAL replay status from Prometheus: %w", err)
	}

	encodedData, err := toonOrJsonOutput(ctx, wal)
	if err != nil {
		return "", fmt.Errorf("error encoding WAL replay status: %w", err)
	}

	return encodedData, nil
}

func cleanTombstonesApiCall(ctx context.Context) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/admin/tsdb/clean_tombstones"
	startTs := time.Now()
	err = client.CleanTombstones(ctx)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error cleaning tombstones from Prometheus: %w", err)
	}

	return "success", nil
}

func deleteSeriesApiCall(ctx context.Context, matches []string, start, end time.Time) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/admin/tsdb/delete_series"
	startTs := time.Now()
	err = client.DeleteSeries(ctx, matches, start, end)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error deleting series from Prometheus: %w", err)
	}

	return "success", nil
}

func snapshotApiCall(ctx context.Context, skipHead bool) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/admin/tsdb/snapshot"
	startTs := time.Now()
	ss, err := client.Snapshot(ctx, skipHead)
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error creating Prometheus snapshot: %w", err)
	}

	encodedData, err := toonOrJsonOutput(ctx, ss)
	if err != nil {
		return "", fmt.Errorf("error encoding snapshot response: %w", err)
	}

	return encodedData, nil
}
