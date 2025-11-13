package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

var (
	resourcePrefix = "prometheus://"

	// Resources.
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

	docsListResource = mcp.NewResource(
		resourcePrefix+"docs",
		"List of Official Prometheus Documentation Files",
		mcp.WithResourceDescription("List of markdown files containing the official Prometheus documentation from the prometheus/docs repo"),
		mcp.WithMIMEType("text/plain"),
	)

	docsReadResourceTemplate = mcp.NewResourceTemplate(
		resourcePrefix+"docs{/file*}",
		"Official Prometheus Documentation",
		mcp.WithTemplateDescription("Read the named markdown file containing official Prometheus documentation from the prometheus/docs repo"),
		mcp.WithTemplateMIMEType("text/markdown"),
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

func docsListResourceHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	docs, err := getDocsFsFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get docs: %w", err)
	}

	names, err := getDocFileNames(docs.fsys)
	if err != nil {
		return nil, fmt.Errorf("failed to list docs: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      resourcePrefix + "list_docs",
			MIMEType: "text/plain",
			Text:     strings.Join(names, "\n"),
		},
	}, nil
}

func docsReadResourceTemplateHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	filenames, ok := request.Params.Arguments["file"].([]string)
	if !ok {
		return nil, errors.New("failed to get filenames to read from resource request")
	}

	if len(filenames) < 1 {
		return nil, errors.New("at least 1 filename is required when requesting docs to read")
	}

	docs, err := getDocsFsFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get docs: %w", err)
	}

	var resourceContents []mcp.ResourceContents
	for _, fname := range filenames {
		content, err := getDocFileContent(docs.fsys, fname)
		if err != nil {
			return nil, fmt.Errorf("error reading file from docs: %w", err)
		}

		resourceContents = append(resourceContents, mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/markdown",
			Text:     content,
		})
	}

	return resourceContents, nil
}
