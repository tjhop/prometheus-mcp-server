package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

var (
	apiTimeout = 1 * time.Minute
)

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
