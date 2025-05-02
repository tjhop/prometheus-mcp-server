package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"

	"github.com/tjhop/prometheus-mcp-server/pkg/prometheus"
)

var (
	// package local Prometheus API client for use with mcp tools/resources/etc
	apiV1Client  v1.API
	apiTimeout   = 1 * time.Minute
	queryTimeout = 30 * time.Second
)

// NewAPIClient creates a new prometheus v1 API client for use by the MCP server
func NewAPIClient() error {
	client, err := prometheus.NewAPIClient()
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
	Result   string      `json:"result"`
	Warnings v1.Warnings `json:"warnings"`
}

func executeQueryApiCall(ctx context.Context, query string) (string, error) {
	result, warnings, err := apiV1Client.Query(ctx, query, time.Now(), v1.WithTimeout(queryTimeout))
	if err != nil {
		return "", fmt.Errorf("error querying Prometheus: %w", err)
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
