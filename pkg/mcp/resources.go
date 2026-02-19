package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	resourcePrefix = "prometheus://"
)

// Resource definitions.
var (
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
	server.AddResource(docsListResource, container.DocsListResourceHandler)

	// Add resource template for reading specific doc files
	server.AddResourceTemplate(docsReadResourceTemplate, container.DocsReadResourceHandler)
}

// Resource handlers

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
