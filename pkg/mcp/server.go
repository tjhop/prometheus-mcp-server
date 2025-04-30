package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"

	"github.com/tjhop/prometheus-mcp-server/internal/version"
	"github.com/tjhop/prometheus-mcp-server/pkg/prometheus"
)

var (
	// package local Prometheus API client for use with mcp tools/resources/etc
	apiV1Client  v1.API
	apiTimeout   = 1 * time.Minute
	queryTimeout = 30 * time.Second

	// Tools
	execQueryTool = mcp.NewTool("execute_query",
		mcp.WithDescription("Execute an instant query against the Prometheus datasource"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Query to be executed"),
		),
		// mcp.WithNumber("timestamp",
		// mcp.Description("Timestamp for the query to be executed at"),
		// ),
	)

	tsdbStatsTool = mcp.NewTool("tsdb_stats",
		mcp.WithDescription("Get usage and cardinality statistics from the TSDB"),
	)
)

// setup pkg local APId
func NewAPIClient() error {
	client, err := prometheus.NewAPIClient()
	if err != nil {
		return fmt.Errorf("failed to create prometheus API client: %w", err)
	}

	apiV1Client = client
	return nil
}

// Handler functions
func execQueryHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	query, ok := arguments["query"].(string)
	if !ok {
		return nil, errors.New("query must be a string")
	}
	if query == "" {
		return nil, errors.New("query cannot be empty")
	}

	// TODO: can client be stored in ctx or passed somehow so it doesn't have to be created every time?
	client, err := api.NewClient(api.Config{
		Address: "http://127.0.0.1:9090",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, apiTimeout) // TODO: make timeout configurable via flag/tool arg?
	defer cancel()

	result, warnings, err := apiV1Client.Query(ctx, query, time.Now(), v1.WithTimeout(queryTimeout))
	if err != nil {
		return nil, fmt.Errorf("error querying Prometheus: %w", err)
	}

	if len(warnings) > 0 {
		// TODO: how do I access the logger? can I?
		fmt.Printf("Warnings: %v\n", warnings)
	}

	toolResult := ""
	if result != nil {
		toolResult = result.String()
	}

	return mcp.NewToolResultText(toolResult), nil
}

func tsdbStatsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	tsdbStats, err := apiV1Client.TSDB(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting tsdb stats from Prometheus: %w", err)
	}

	jsonBytes, err := json.Marshal(tsdbStats)
	if err != nil {
		return nil, fmt.Errorf("error converting tsdb stats to JSON: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

func NewServer(logger *slog.Logger) *server.MCPServer {
	hooks := &server.Hooks{}

	hooks.AddBeforeCallTool(func(ctx context.Context, id any, message *mcp.CallToolRequest) {
		logger.Debug("Before Call Tool", "id", id, "tool_request", message)
	})

	mcpServer := server.NewMCPServer(
		"prometheus-mcp-server",
		version.Info(),
		server.WithLogging(),
		server.WithHooks(hooks),
	)

	mcpServer.AddTool(execQueryTool, execQueryHandler)
	mcpServer.AddTool(tsdbStatsTool, tsdbStatsHandler)

	return mcpServer
}
