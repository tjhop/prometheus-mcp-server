package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alpkeskin/gotoon"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tjhop/prometheus-mcp-server/internal/metrics"
	mcpProm "github.com/tjhop/prometheus-mcp-server/pkg/prometheus"
)

var (
	DefaultLookbackDelta = -5 * time.Minute

	apiTimeout   = 1 * time.Minute
	queryTimeout = 30 * time.Second

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

func queryApiCall(ctx context.Context, query string, ts time.Time) (string, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting API client from context: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/query"
	startTs := time.Now()
	result, warnings, err := client.Query(ctx, query, ts, promv1.WithTimeout(queryTimeout))
	metricApiCallDuration.With(prometheus.Labels{"target_path": path}).Observe(time.Since(startTs).Seconds())
	if err != nil {
		metricApiCallsFailed.With(prometheus.Labels{"target_path": path}).Inc()
		return "", fmt.Errorf("error executing instant query: %w", err)
	}

	res := queryApiResponse{
		Result:   result.String(),
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

	res := queryApiResponse{
		Result:   result.String(),
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

	encodedData, err := toonOrJsonOutput(ctx, res)
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

	res := queryApiResponse{
		Result:   strings.Join(lsets, "\n"),
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

	res := queryApiResponse{
		Result:   strings.Join(result, "\n"),
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

	res := queryApiResponse{
		Result:   strings.Join(lvals, "\n"),
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
