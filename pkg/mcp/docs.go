package mcp

import (
	"context"
	"embed"
	"errors"
	"io/fs"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var (
	//go:embed assets/*
	assets embed.FS
)

// Context key and middlewares for embedding Prometheus' docs as an fs.FS into
// a context for use with tool/resource calls. Avoids the need for
// global/external state to maintain the docs FS otherwise.
type docsKey struct{}
type docsLoaderMiddleware struct {
	fsys fs.FS
}

func newDocsLoaderMiddleware(fsys fs.FS) *docsLoaderMiddleware {
	docsMW := docsLoaderMiddleware{
		fsys: fsys,
	}

	return &docsMW
}

func addDocsToContext(ctx context.Context, fsys fs.FS) context.Context {
	return context.WithValue(ctx, docsKey{}, fsys)
}

func getDocsFsFromContext(ctx context.Context) (fs.FS, error) {
	docs, ok := ctx.Value(docsKey{}).(fs.FS)
	if !ok {
		return nil, errors.New("failed to get docs FS from context")
	}

	return docs, nil
}

func (m *docsLoaderMiddleware) ToolMiddleware(next server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return next(addDocsToContext(ctx, m.fsys), req)
	}
}

func (m *docsLoaderMiddleware) ResourceMiddleware(next server.ResourceHandlerFunc) server.ResourceHandlerFunc {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return next(addDocsToContext(ctx, m.fsys), request)
	}
}
