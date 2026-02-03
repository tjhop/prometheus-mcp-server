package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	resourcePrefix = "prometheus://"
)

// Resource definitions.
var (
	listMetricsResource = &mcp.Resource{
		URI:         resourcePrefix + "list_metrics",
		Name:        "List metrics",
		Description: "List metrics available",
		MIMEType:    "application/json",
	}

	targetsResource = &mcp.Resource{
		URI:         resourcePrefix + "targets",
		Name:        "Targets",
		Description: "Overview of the current state of the Prometheus target discovery",
		MIMEType:    "application/json",
	}

	tsdbStatsResource = &mcp.Resource{
		URI:         resourcePrefix + "tsdb_stats",
		Name:        "TSDB Stats",
		Description: "Usage and cardinality statistics from the TSDB",
		MIMEType:    "application/json",
	}

	docsListResource = &mcp.Resource{
		URI:         resourcePrefix + "docs",
		Name:        "List of Official Prometheus Documentation Files",
		Description: "List of markdown files containing the official Prometheus documentation from the prometheus/docs repo",
		MIMEType:    "text/plain",
	}

	docsReadResourceTemplate = &mcp.ResourceTemplate{
		URITemplate: resourcePrefix + "docs/{+file}",
		Name:        "Official Prometheus Documentation",
		Description: "Read the named markdown file containing official Prometheus documentation from the prometheus/docs repo",
		MIMEType:    "text/markdown",
	}
)

// registerResources registers all MCP resources with the server.
func registerResources(server *mcp.Server, container *ServerContainer) {
	// Add static resources
	server.AddResource(listMetricsResource, container.ListMetricsResourceHandler)
	server.AddResource(targetsResource, container.TargetsResourceHandler)
	server.AddResource(tsdbStatsResource, container.TsdbStatsResourceHandler)
	server.AddResource(docsListResource, container.DocsListResourceHandler)

	// Add resource template for reading specific doc files
	server.AddResourceTemplate(docsReadResourceTemplate, container.DocsReadResourceHandler)
}

// Resource handlers

// ListMetricsResourceHandler handles the list_metrics resource request.
func (s *ServerContainer) ListMetricsResourceHandler(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	result, err := s.labelValuesAPICall(ctx, "__name__", nil, time.Time{}, time.Time{}, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get metric names: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     result,
			},
		},
	}, nil
}

// TargetsResourceHandler handles the targets resource request.
func (s *ServerContainer) TargetsResourceHandler(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	result, err := s.targetsAPICall(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get target info: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     result,
			},
		},
	}, nil
}

// TsdbStatsResourceHandler handles the tsdb_stats resource request.
func (s *ServerContainer) TsdbStatsResourceHandler(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	result, err := s.tsdbStatsAPICall(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to process tsdb stats: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     result,
			},
		},
	}, nil
}

// DocsListResourceHandler handles the docs list resource request.
func (s *ServerContainer) DocsListResourceHandler(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	names, err := s.GetDocFileNames()
	if err != nil {
		return nil, fmt.Errorf("failed to list docs: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "text/plain",
				Text:     strings.Join(names, "\n"),
			},
		},
	}, nil
}

// DocsReadResourceHandler handles reading specific documentation files via the resource template.
func (s *ServerContainer) DocsReadResourceHandler(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	// Parse the URI to properly handle URL-encoded characters.
	u, err := url.Parse(req.Params.URI)
	if err != nil {
		return nil, fmt.Errorf("invalid resource URI: %w", err)
	}

	// Validate scheme.
	if u.Scheme != "prometheus" {
		return nil, fmt.Errorf("invalid docs resource URI scheme: %s", u.Scheme)
	}

	// Get file path. Ie, prometheus://docs/file/to-read.md -> file/to-read.md.
	filename := strings.TrimPrefix(u.Path, "/")
	if filename == "" {
		return nil, errors.New("at least 1 filename is required when requesting docs to read")
	}

	content, err := s.GetDocFileContent(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file from docs: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "text/markdown",
				Text:     content,
			},
		},
	}, nil
}
