package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

var (
	// Resources
	resourcePrefix = "prometheus://"

	listMetricsResource = mcp.NewResource(
		resourcePrefix+"list_metrics",
		"List metrics",
		mcp.WithResourceDescription("List metrics available"),
		mcp.WithMIMEType("application/json"),
	)

	targetsResource = mcp.NewResource(
		resourcePrefix+"targets",
		"Targets",
		mcp.WithResourceDescription("Overview of the current state of the Prometheus target discovery"),
		mcp.WithMIMEType("application/json"),
	)

	tsdbStatsResource = mcp.NewResource(
		resourcePrefix+"tsdb_stats",
		"TSDB Stats",
		mcp.WithResourceDescription("Usage and cardinality statistics from the TSDB"),
		mcp.WithMIMEType("application/json"),
	)
)

func listMetricsResourceHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	metrics, err := labelValuesApiCall(ctx, "__name__", nil, time.Time{}, time.Time{})
	if err != nil {
		return nil, fmt.Errorf("error getting metric names: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      resourcePrefix + "list_metrics",
			MIMEType: "application/json",
			Text:     metrics,
		},
	}, nil
}

func targetsResourceHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	targets, err := targetsApiCall(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting target info: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      resourcePrefix + "targets",
			MIMEType: "application/json",
			Text:     targets,
		},
	}, nil
}

func tsdbStatsResourceHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	tsdbStats, err := tsdbStatsApiCall(ctx)
	if err != nil {
		return nil, fmt.Errorf("error processing tsdb stats: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      resourcePrefix + "tsdb_stats",
			MIMEType: "application/json",
			Text:     tsdbStats,
		},
	}, nil
}
