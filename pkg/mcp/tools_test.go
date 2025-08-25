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

func TestRangeQueryToolHandler(t *testing.T) {
	testCases := []struct {
		name           string
		request        mcp.CallToolRequest
		mockQueryFunc  func(ctx context.Context, query string, r promv1.Range, opts ...promv1.Option) (model.Value, promv1.Warnings, error)
		validateResult func(t *testing.T, result *mcp.CallToolResult, err error)
	}{
		{
			name: "success",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "range_query",
					Arguments: map[string]any{
						"query":      "up",
						"start_time": "1756142748",
						"end_time":   "1756143048",
					},
				},
			},
			mockQueryFunc: func(ctx context.Context, query string, r promv1.Range, opts ...promv1.Option) (model.Value, promv1.Warnings, error) {
				require.Equal(t, "up", query)
				return model.Matrix{}, nil, nil
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsError)
			},
		},
		{
			name: "missing query",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name:      "range_query",
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
					Name:      "range_query",
					Arguments: map[string]any{"query": "up"},
				},
			},
			mockQueryFunc: func(ctx context.Context, query string, r promv1.Range, opts ...promv1.Option) (model.Value, promv1.Warnings, error) {
				return nil, nil, errors.New("prometheus exploded")
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, toolCallResultAsString(result), "prometheus exploded")
			},
		},
		{
			name: "invalid start_time",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name:      "range_query",
					Arguments: map[string]any{"query": "up", "start_time": "not-a-real-timestamp"},
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, toolCallResultAsString(result), "failed to parse start_time")
			},
		},
		{
			name: "invalid end_time",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name:      "range_query",
					Arguments: map[string]any{"query": "up", "end_time": "not-a-real-timestamp"},
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, toolCallResultAsString(result), "failed to parse end_time")
			},
		},
		{
			name: "invalid step",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name:      "range_query",
					Arguments: map[string]any{"query": "up", "step": "not-a-real-duration"},
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, toolCallResultAsString(result), "failed to parse duration")
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(rangeQueryTool, rangeQueryToolHandler)

	ctx := context.WithValue(context.Background(), apiClientKey{}, mockAPI)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.QueryRangeFunc = tc.mockQueryFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
			require.NoError(t, err)

			tc.validateResult(t, res, err)
		})
	}
}

func TestSnapshotToolHandler(t *testing.T) {
	testCases := []struct {
		name             string
		request          mcp.CallToolRequest
		mockSnapshotFunc func(ctx context.Context, skipHead bool) (promv1.SnapshotResult, error)
		validateResult   func(t *testing.T, result *mcp.CallToolResult, err error)
	}{
		{
			name: "success",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "snapshot",
					Arguments: map[string]any{
						"skip_head": true,
					},
				},
			},
			mockSnapshotFunc: func(ctx context.Context, skipHead bool) (promv1.SnapshotResult, error) {
				require.True(t, skipHead)
				return promv1.SnapshotResult{}, nil
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsError)
			},
		},
		{
			name: "API error",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "snapshot",
				},
			},
			mockSnapshotFunc: func(ctx context.Context, skipHead bool) (promv1.SnapshotResult, error) {
				return promv1.SnapshotResult{}, errors.New("prometheus exploded")
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, toolCallResultAsString(result), "prometheus exploded")
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(snapshotTool, snapshotToolHandler)

	ctx := context.WithValue(context.Background(), apiClientKey{}, mockAPI)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.SnapshotFunc = tc.mockSnapshotFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
			require.NoError(t, err)

			tc.validateResult(t, res, err)
		})
	}
}

func TestDeleteSeriesToolHandler(t *testing.T) {
	testCases := []struct {
		name                 string
		request              mcp.CallToolRequest
		mockDeleteSeriesFunc func(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) error
		validateResult       func(t *testing.T, result *mcp.CallToolResult, err error)
	}{
		{
			name: "success",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "delete_series",
					Arguments: map[string]any{
						"matches":    []string{"up"},
						"start_time": "1756142748",
						"end_time":   "1756143048",
					},
				},
			},
			mockDeleteSeriesFunc: func(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) error {
				require.Equal(t, []string{"up"}, matches)
				return nil
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsError)
			},
		},
		{
			name: "missing matches",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name:      "delete_series",
					Arguments: map[string]any{},
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, toolCallResultAsString(result), "matches must be an array")
			},
		},
		{
			name: "API error",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name:      "delete_series",
					Arguments: map[string]any{"matches": []string{"up"}},
				},
			},
			mockDeleteSeriesFunc: func(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) error {
				return errors.New("prometheus exploded")
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, toolCallResultAsString(result), "prometheus exploded")
			},
		},
		{
			name: "invalid start_time",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name:      "delete_series",
					Arguments: map[string]any{"matches": []string{"up"}, "start_time": "not-a-real-timestamp"},
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, toolCallResultAsString(result), "failed to parse start_time")
			},
		},
		{
			name: "invalid end_time",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name:      "delete_series",
					Arguments: map[string]any{"matches": []string{"up"}, "end_time": "not-a-real-timestamp"},
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, toolCallResultAsString(result), "failed to parse end_time")
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(deleteSeriesTool, deleteSeriesToolHandler)

	ctx := context.WithValue(context.Background(), apiClientKey{}, mockAPI)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.DeleteSeriesFunc = tc.mockDeleteSeriesFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
			require.NoError(t, err)

			tc.validateResult(t, res, err)
		})
	}
}
