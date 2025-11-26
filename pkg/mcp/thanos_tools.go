package mcp

import (
	"context"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
)

var (
	thanosStoresTool = mcp.NewTool("list_stores",
		mcp.WithDescription("List all store API servers"),
	)
)

func thanosStoresToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := getApiClientFromContext(ctx)
	if err != nil {
		return mcp.NewToolResultError("error getting prometheus api client from context: " + err.Error()), nil
	}
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	path := "/api/v1/stores"
	data, err := doHttpRequest(ctx, http.MethodGet, client.roundtripper, client.url, path, true)
	if err != nil {
		return mcp.NewToolResultError("error getting stores from Thanos: " + err.Error()), nil
	}

	return mcp.NewToolResultText(data), nil
}
