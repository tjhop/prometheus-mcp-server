package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func toolCallResultAsString(result *mcp.CallToolResult) string {
	var output strings.Builder
	for _, c := range result.Content {
		if text, ok := c.(mcp.TextContent); ok {
			output.WriteString(text.Text)
		}
	}

	return output.String()
}

func TestQueryToolHandler(t *testing.T) {
	testCases := []struct {
		name           string
		request        mcp.CallToolRequest
		mockQueryFunc  func(ctx context.Context, query string, ts time.Time, opts ...promv1.Option) (model.Value, promv1.Warnings, error)
		validateResult func(t *testing.T, result *mcp.CallToolResult, err error)
	}{
		{
			name: "success",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "query",
					Arguments: map[string]any{
						"query":     "vector(1)",
						"timestamp": "1756143048",
					},
				},
			},
			mockQueryFunc: func(ctx context.Context, query string, ts time.Time, opts ...promv1.Option) (model.Value, promv1.Warnings, error) {
				require.Equal(t, "vector(1)", query)
				return model.Vector{&model.Sample{
					Metric:    model.Metric{},
					Value:     model.SampleValue(1),
					Timestamp: model.TimeFromUnix(ts.Unix()),
				}}, nil, nil
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsError)
				require.Equal(t, `{"result":"{} =\u003e 1 @[1756143048]","warnings":null}`, toolCallResultAsString(result))
			},
		},
		{
			name: "missing query",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name:      "query",
					Arguments: map[string]any{},
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, toolCallResultAsString(result), "query must be a string")
			},
		},
		{
			name: "API error",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name:      "query",
					Arguments: map[string]any{"query": "up"},
				},
			},
			mockQueryFunc: func(ctx context.Context, query string, ts time.Time, opts ...promv1.Option) (model.Value, promv1.Warnings, error) {
				return nil, nil, errors.New("prometheus exploded")
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, toolCallResultAsString(result), "prometheus exploded")
			},
		},
		{
			name: "invalid timestamp",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name:      "query",
					Arguments: map[string]any{"query": "up", "timestamp": "not-a-real-timestamp"},
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, toolCallResultAsString(result), "failed to get ts from args")
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(queryTool, queryToolHandler)

	ctx := context.WithValue(context.Background(), apiClientKey{}, mockAPI)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.QueryFunc = tc.mockQueryFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
			require.NoError(t, err)

			tc.validateResult(t, res, err)
		})
	}
}
