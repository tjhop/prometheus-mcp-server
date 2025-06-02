package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"

	"github.com/tjhop/prometheus-mcp-server/pkg/prometheus"
)

var (
	DefaultLookbackDelta = -5 * time.Minute

	// package local Prometheus API client for use with mcp tools/resources/etc
	apiV1Client  promv1.API
	apiTimeout   = 1 * time.Minute
	queryTimeout = 30 * time.Second
)

// NewAPIClient creates a new prometheus v1 API client for use by the MCP server
func NewAPIClient(prometheusUrl, httpConfig string) error {
	client, err := prometheus.NewAPIClient(prometheusUrl, httpConfig)
	if err != nil {
		return fmt.Errorf("failed to create prometheus API client: %w", err)
	}

	apiV1Client = client
	return nil
}

func alertmanagersApiCall(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	ams, err := apiV1Client.AlertManagers(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting alertmanager status from Prometheus: %w", err)
	}

	jsonBytes, err := json.Marshal(ams)
	if err != nil {
		return "", fmt.Errorf("error converting alertmanager status to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

func flagsApiCall(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	flags, err := apiV1Client.Flags(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting runtime flags from Prometheus: %w", err)
	}

	jsonBytes, err := json.Marshal(flags)
	if err != nil {
		return "", fmt.Errorf("error converting runtime flags to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

func listAlertsApiCall(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	alerts, err := apiV1Client.Alerts(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting alerts from Prometheus: %w", err)
	}

	jsonBytes, err := json.Marshal(alerts)
	if err != nil {
		return "", fmt.Errorf("error converting alerts to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

func tsdbStatsApiCall(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	tsdbStats, err := apiV1Client.TSDB(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting tsdb stats from Prometheus: %w", err)
	}

	jsonBytes, err := json.Marshal(tsdbStats)
	if err != nil {
		return "", fmt.Errorf("error converting tsdb stats to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

type queryApiResponse struct {
	Result   string          `json:"result"`
	Warnings promv1.Warnings `json:"warnings"`
}

func queryApiCall(ctx context.Context, query string, ts time.Time) (string, error) {
	result, warnings, err := apiV1Client.Query(ctx, query, ts, promv1.WithTimeout(queryTimeout))
	if err != nil {
		return "", fmt.Errorf("error executing instant query: %w", err)
	}

	res := queryApiResponse{
		Result:   result.String(),
		Warnings: warnings,
	}

	jsonBytes, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("error converting query response to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

func rangeQueryApiCall(ctx context.Context, query string, start, end time.Time, step time.Duration) (string, error) {
	result, warnings, err := apiV1Client.QueryRange(ctx, query, promv1.Range{Start: start, End: end, Step: step})
	if err != nil {
		return "", fmt.Errorf("error executing range query: %w", err)
	}

	res := queryApiResponse{
		Result:   result.String(),
		Warnings: warnings,
	}

	jsonBytes, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("error converting query response to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

func seriesApiCall(ctx context.Context, matchers []string, start, end time.Time) (string, error) {
	result, warnings, err := apiV1Client.Series(ctx, matchers, start, end)
	if err != nil {
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

	jsonBytes, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("error converting series response to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

func labelNamesApiCall(ctx context.Context, matchers []string, start, end time.Time) (string, error) {
	result, warnings, err := apiV1Client.LabelNames(ctx, matchers, start, end)
	if err != nil {
		return "", fmt.Errorf("error getting label names: %w", err)
	}

	res := queryApiResponse{
		Result:   strings.Join(result, "\n"),
		Warnings: warnings,
	}

	jsonBytes, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("error converting label names response to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

func labelValuesApiCall(ctx context.Context, label string, matchers []string, start, end time.Time) (string, error) {
	result, warnings, err := apiV1Client.LabelValues(ctx, label, matchers, start, end)
	if err != nil {
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

	jsonBytes, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("error converting label values response to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

func buildinfoApiCall(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	bi, err := apiV1Client.Buildinfo(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting build info from Prometheus: %w", err)
	}

	jsonBytes, err := json.Marshal(bi)
	if err != nil {
		return "", fmt.Errorf("error converting build info to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

func configApiCall(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	cfg, err := apiV1Client.Config(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting configuration from Prometheus: %w", err)
	}

	jsonBytes, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("error converting configuration to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

func runtimeinfoApiCall(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	ri, err := apiV1Client.Runtimeinfo(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting runtime info from Prometheus: %w", err)
	}

	jsonBytes, err := json.Marshal(ri)
	if err != nil {
		return "", fmt.Errorf("error converting runtime info to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

func rulesApiCall(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	rules, err := apiV1Client.Rules(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting rules from Prometheus: %w", err)
	}

	jsonBytes, err := json.Marshal(rules)
	if err != nil {
		return "", fmt.Errorf("error converting rules to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

func targetsApiCall(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	targets, err := apiV1Client.Targets(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting targets from Prometheus: %w", err)
	}

	jsonBytes, err := json.Marshal(targets)
	if err != nil {
		return "", fmt.Errorf("error converting targets to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

func targetsMetadataApiCall(ctx context.Context, matchTarget, metric, limit string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	tm, err := apiV1Client.TargetsMetadata(ctx, matchTarget, metric, limit)
	if err != nil {
		return "", fmt.Errorf("error getting target metadata from Prometheus: %w", err)
	}

	jsonBytes, err := json.Marshal(tm)
	if err != nil {
		return "", fmt.Errorf("error converting target metadata to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

func walReplayApiCall(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	wal, err := apiV1Client.WalReplay(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting WAL replay status from Prometheus: %w", err)
	}

	jsonBytes, err := json.Marshal(wal)
	if err != nil {
		return "", fmt.Errorf("error converting WAL replay status to JSON: %w", err)
	}

	return string(jsonBytes), nil
}
