package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/mark3labs/mcp-go/server"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

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
				require.JSONEq(t, `{"result":"{} =\u003e 1 @[1756143048]","warnings":null}`, getToolCallResultAsString(result))
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
				require.Contains(t, getToolCallResultAsString(result), "query must be a string")
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
				require.Contains(t, getToolCallResultAsString(result), "prometheus exploded")
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
				require.Contains(t, getToolCallResultAsString(result), "failed to get ts from args")
			},
		},
		{
			name: "truncation - truncated output with warning",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "query",
					Arguments: map[string]any{
						"query":            "vector(1)",
						"timestamp":        "1756143048",
						"truncation_limit": 1,
					},
				},
			},
			mockQueryFunc: func(ctx context.Context, query string, ts time.Time, opts ...promv1.Option) (model.Value, promv1.Warnings, error) {
				require.Equal(t, "vector(1)", query)
				return model.Vector{&model.Sample{
					Metric:    model.Metric{},
					Value:     model.SampleValue(1),
					Timestamp: model.TimeFromUnix(ts.Unix()),
				}, &model.Sample{
					Metric:    model.Metric{},
					Value:     model.SampleValue(2),
					Timestamp: model.TimeFromUnix(ts.Unix()),
				}}, nil, nil
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsError)
				expectedWarning := strings.ReplaceAll(displayTruncationWarning(1), "\n", "\\n")
				expectedResult := fmt.Sprintf(`{"result":"{} =\u003e 1 @[1756143048]%s","warnings":null}`, expectedWarning)
				require.JSONEq(t, expectedResult, getToolCallResultAsString(result))
			},
		},
		{
			name: "truncation - not truncated, no warning",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "query",
					Arguments: map[string]any{
						"query":            "vector(1)",
						"timestamp":        "1756143048",
						"truncation_limit": 0,
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
				require.JSONEq(t, `{"result":"{} =\u003e 1 @[1756143048]","warnings":null}`, getToolCallResultAsString(result))
			},
		},
	}
	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(prometheusQueryTool, prometheusQueryToolHandler)

	promApi := promApi{
		API: mockAPI,
	}
	ctx := addApiClientToContext(context.Background(), promApi)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.QueryFunc = tc.mockQueryFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
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
				require.Contains(t, getToolCallResultAsString(result), "query must be a string")
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
				require.Contains(t, getToolCallResultAsString(result), "prometheus exploded")
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
				require.Contains(t, getToolCallResultAsString(result), "failed to parse start_time")
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
				require.Contains(t, getToolCallResultAsString(result), "failed to parse end_time")
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
				require.Contains(t, getToolCallResultAsString(result), "failed to parse duration")
			},
		},
		{
			name: "truncation - truncated output with warning",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "range_query",
					Arguments: map[string]any{
						"query":            "up",
						"start_time":       "1756142748",
						"end_time":         "1756143048",
						"truncation_limit": 1,
					},
				},
			},
			mockQueryFunc: func(ctx context.Context, query string, r promv1.Range, opts ...promv1.Option) (model.Value, promv1.Warnings, error) {
				require.Equal(t, "up", query)
				mockMatrix := model.Matrix{
					&model.SampleStream{
						Metric: model.Metric{"__name__": "up", "instance": "localhost:9090", "job": "prometheus"},
						Values: []model.SamplePair{
							{Timestamp: model.Time(1756142748), Value: 1},
							{Timestamp: model.Time(1756142749), Value: 1},
						},
					},
					&model.SampleStream{
						Metric: model.Metric{"__name__": "up", "instance": "localhost:9100", "job": "node-exporter"},
						Values: []model.SamplePair{
							{Timestamp: model.Time(1756142748), Value: 1},
							{Timestamp: model.Time(1756142749), Value: 1},
						},
					},
				}
				return mockMatrix, nil, nil
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsError)
				expectedWarning := strings.ReplaceAll(displayTruncationWarning(1), "\n", "\\n")
				expectedResult := fmt.Sprintf(`{"result":"up{instance=\u0022localhost:9090\u0022, job=\u0022prometheus\u0022} =\u003e%s","warnings":null}`, expectedWarning)
				require.JSONEq(t, expectedResult, getToolCallResultAsString(result))
			},
		},
		{
			name: "truncation - not truncated, no warning",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "range_query",
					Arguments: map[string]any{
						"query":            "up",
						"start_time":       "1756142748",
						"end_time":         "1756143048",
						"truncation_limit": 0,
					},
				},
			},
			mockQueryFunc: func(ctx context.Context, query string, r promv1.Range, opts ...promv1.Option) (model.Value, promv1.Warnings, error) {
				require.Equal(t, "up", query)
				mockMatrix := model.Matrix{
					&model.SampleStream{
						Metric: model.Metric{"__name__": "up", "instance": "localhost:9090", "job": "prometheus"},
						Values: []model.SamplePair{
							{Timestamp: model.Time(1756142748), Value: 1},
							{Timestamp: model.Time(1756142749), Value: 1},
						},
					},
					&model.SampleStream{
						Metric: model.Metric{"__name__": "up", "instance": "localhost:9100", "job": "node-exporter"},
						Values: []model.SamplePair{
							{Timestamp: model.Time(1756142748), Value: 1},
							{Timestamp: model.Time(1756142749), Value: 1},
						},
					},
				}
				return mockMatrix, nil, nil
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsError)
				expectedResult := `{"result":"up{instance=\u0022localhost:9090\u0022, job=\u0022prometheus\u0022} =\u003e\n1 @[1756142.748]\n1 @[1756142.749]\nup{instance=\"localhost:9100\u0022, job=\u0022node-exporter\u0022} =\u003e\n1 @[1756142.748]\n1 @[1756142.749]","warnings":null}`
				require.JSONEq(t, expectedResult, getToolCallResultAsString(result))
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(prometheusRangeQueryTool, prometheusRangeQueryToolHandler)

	promApi := promApi{
		API: mockAPI,
	}
	ctx := addApiClientToContext(context.Background(), promApi)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.QueryRangeFunc = tc.mockQueryFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
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
				require.Contains(t, getToolCallResultAsString(result), "prometheus exploded")
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(prometheusSnapshotTool, prometheusSnapshotToolHandler)

	promApi := promApi{
		API: mockAPI,
	}
	ctx := addApiClientToContext(context.Background(), promApi)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.SnapshotFunc = tc.mockSnapshotFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
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
				require.Contains(t, getToolCallResultAsString(result), "matches must be an array")
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
				require.Contains(t, getToolCallResultAsString(result), "prometheus exploded")
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
				require.Contains(t, getToolCallResultAsString(result), "failed to parse start_time")
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
				require.Contains(t, getToolCallResultAsString(result), "failed to parse end_time")
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(prometheusDeleteSeriesTool, prometheusDeleteSeriesToolHandler)

	promApi := promApi{
		API: mockAPI,
	}
	ctx := addApiClientToContext(context.Background(), promApi)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.DeleteSeriesFunc = tc.mockDeleteSeriesFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
			tc.validateResult(t, res, err)
		})
	}
}

func TestCleanTombstonesToolHandler(t *testing.T) {
	testCases := []struct {
		name                    string
		request                 mcp.CallToolRequest
		mockCleanTombstonesFunc func(ctx context.Context) error
		validateResult          func(t *testing.T, result *mcp.CallToolResult, err error)
	}{
		{
			name: "success",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "clean_tombstones",
				},
			},
			mockCleanTombstonesFunc: func(ctx context.Context) error {
				return nil
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
					Name: "clean_tombstones",
				},
			},
			mockCleanTombstonesFunc: func(ctx context.Context) error {
				return errors.New("prometheus exploded")
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "prometheus exploded")
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(prometheusCleanTombstonesTool, prometheusCleanTombstonesToolHandler)

	promApi := promApi{
		API: mockAPI,
	}
	ctx := addApiClientToContext(context.Background(), promApi)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.CleanTombstonesFunc = tc.mockCleanTombstonesFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
			tc.validateResult(t, res, err)
		})
	}
}

func TestMetricMetadataToolHandler(t *testing.T) {
	testCases := []struct {
		name             string
		request          mcp.CallToolRequest
		mockMetadataFunc func(ctx context.Context, metric string, limit string) (map[string][]promv1.Metadata, error)
		validateResult   func(t *testing.T, result *mcp.CallToolResult, err error)
	}{
		{
			name: "success with params",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "metric_metadata",
					Arguments: map[string]any{
						"metric": "go_goroutines",
						"limit":  "10",
					},
				},
			},
			mockMetadataFunc: func(ctx context.Context, metric string, limit string) (map[string][]promv1.Metadata, error) {
				require.Equal(t, "go_goroutines", metric)
				require.Equal(t, "10", limit)
				return map[string][]promv1.Metadata{}, nil
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsError)
			},
		},
		{
			name: "success no params",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "metric_metadata",
				},
			},
			mockMetadataFunc: func(ctx context.Context, metric string, limit string) (map[string][]promv1.Metadata, error) {
				return map[string][]promv1.Metadata{}, nil
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
					Name: "metric_metadata",
				},
			},
			mockMetadataFunc: func(ctx context.Context, metric string, limit string) (map[string][]promv1.Metadata, error) {
				return nil, errors.New("prometheus exploded")
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "prometheus exploded")
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(prometheusMetricMetadataTool, prometheusMetricMetadataToolHandler)

	promApi := promApi{
		API: mockAPI,
	}
	ctx := addApiClientToContext(context.Background(), promApi)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.MetadataFunc = tc.mockMetadataFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
			tc.validateResult(t, res, err)
		})
	}
}

func TestTargetsMetadataToolHandler(t *testing.T) {
	testCases := []struct {
		name                    string
		request                 mcp.CallToolRequest
		mockTargetsMetadataFunc func(ctx context.Context, matchTarget string, metric string, limit string) ([]promv1.MetricMetadata, error)
		validateResult          func(t *testing.T, result *mcp.CallToolResult, err error)
	}{
		{
			name: "success with params",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "targets_metadata",
					Arguments: map[string]any{
						"match_target": `{job="prometheus"}`,
						"metric":       "go_goroutines",
						"limit":        "10",
					},
				},
			},
			mockTargetsMetadataFunc: func(ctx context.Context, matchTarget string, metric string, limit string) ([]promv1.MetricMetadata, error) {
				require.Equal(t, `{job="prometheus"}`, matchTarget)
				require.Equal(t, "go_goroutines", metric)
				require.Equal(t, "10", limit)
				return []promv1.MetricMetadata{}, nil
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsError)
			},
		},
		{
			name: "success no params",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "targets_metadata",
				},
			},
			mockTargetsMetadataFunc: func(ctx context.Context, matchTarget string, metric string, limit string) ([]promv1.MetricMetadata, error) {
				return []promv1.MetricMetadata{}, nil
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
					Name: "targets_metadata",
				},
			},
			mockTargetsMetadataFunc: func(ctx context.Context, matchTarget string, metric string, limit string) ([]promv1.MetricMetadata, error) {
				return nil, errors.New("prometheus exploded")
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "prometheus exploded")
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(prometheusTargetsMetadataTool, prometheusTargetsMetadataToolHandler)

	promApi := promApi{
		API: mockAPI,
	}
	ctx := addApiClientToContext(context.Background(), promApi)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.TargetsMetadataFunc = tc.mockTargetsMetadataFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
			tc.validateResult(t, res, err)
		})
	}
}

func TestListTargetsToolHandler(t *testing.T) {
	testCases := []struct {
		name            string
		request         mcp.CallToolRequest
		mockTargetsFunc func(ctx context.Context) (promv1.TargetsResult, error)
		validateResult  func(t *testing.T, result *mcp.CallToolResult, err error)
	}{
		{
			name: "success",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "list_targets",
				},
			},
			mockTargetsFunc: func(ctx context.Context) (promv1.TargetsResult, error) {
				return promv1.TargetsResult{}, nil
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
					Name: "list_targets",
				},
			},
			mockTargetsFunc: func(ctx context.Context) (promv1.TargetsResult, error) {
				return promv1.TargetsResult{}, errors.New("prometheus exploded")
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "prometheus exploded")
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(prometheusTargetsTool, prometheusTargetsToolHandler)

	promApi := promApi{
		API: mockAPI,
	}
	ctx := addApiClientToContext(context.Background(), promApi)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.TargetsFunc = tc.mockTargetsFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
			tc.validateResult(t, res, err)
		})
	}
}

func TestListRulesToolHandler(t *testing.T) {
	testCases := []struct {
		name           string
		request        mcp.CallToolRequest
		mockRulesFunc  func(ctx context.Context) (promv1.RulesResult, error)
		validateResult func(t *testing.T, result *mcp.CallToolResult, err error)
	}{
		{
			name: "success",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "list_rules",
				},
			},
			mockRulesFunc: func(ctx context.Context) (promv1.RulesResult, error) {
				return promv1.RulesResult{}, nil
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
					Name: "list_rules",
				},
			},
			mockRulesFunc: func(ctx context.Context) (promv1.RulesResult, error) {
				return promv1.RulesResult{}, errors.New("prometheus exploded")
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "prometheus exploded")
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(prometheusRulesTool, prometheusRulesToolHandler)

	promApi := promApi{
		API: mockAPI,
	}
	ctx := addApiClientToContext(context.Background(), promApi)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.RulesFunc = tc.mockRulesFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
			tc.validateResult(t, res, err)
		})
	}
}

func TestRuntimeinfoToolHandler(t *testing.T) {
	testCases := []struct {
		name                string
		request             mcp.CallToolRequest
		mockRuntimeinfoFunc func(ctx context.Context) (promv1.RuntimeinfoResult, error)
		validateResult      func(t *testing.T, result *mcp.CallToolResult, err error)
	}{
		{
			name: "success",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "runtime_info",
				},
			},
			mockRuntimeinfoFunc: func(ctx context.Context) (promv1.RuntimeinfoResult, error) {
				return promv1.RuntimeinfoResult{}, nil
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
					Name: "runtime_info",
				},
			},
			mockRuntimeinfoFunc: func(ctx context.Context) (promv1.RuntimeinfoResult, error) {
				return promv1.RuntimeinfoResult{}, errors.New("prometheus exploded")
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "prometheus exploded")
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(prometheusRuntimeinfoTool, prometheusRuntimeinfoToolHandler)

	promApi := promApi{
		API: mockAPI,
	}
	ctx := addApiClientToContext(context.Background(), promApi)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.RuntimeinfoFunc = tc.mockRuntimeinfoFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
			tc.validateResult(t, res, err)
		})
	}
}

func TestConfigToolHandler(t *testing.T) {
	testCases := []struct {
		name           string
		request        mcp.CallToolRequest
		mockConfigFunc func(ctx context.Context) (promv1.ConfigResult, error)
		validateResult func(t *testing.T, result *mcp.CallToolResult, err error)
	}{
		{
			name: "success",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "config",
				},
			},
			mockConfigFunc: func(ctx context.Context) (promv1.ConfigResult, error) {
				return promv1.ConfigResult{}, nil
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
					Name: "config",
				},
			},
			mockConfigFunc: func(ctx context.Context) (promv1.ConfigResult, error) {
				return promv1.ConfigResult{}, errors.New("prometheus exploded")
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "prometheus exploded")
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(prometheusConfigTool, prometheusConfigToolHandler)

	promApi := promApi{
		API: mockAPI,
	}
	ctx := addApiClientToContext(context.Background(), promApi)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.ConfigFunc = tc.mockConfigFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
			tc.validateResult(t, res, err)
		})
	}
}

func TestBuildinfoToolHandler(t *testing.T) {
	testCases := []struct {
		name              string
		request           mcp.CallToolRequest
		mockBuildinfoFunc func(ctx context.Context) (promv1.BuildinfoResult, error)
		validateResult    func(t *testing.T, result *mcp.CallToolResult, err error)
	}{
		{
			name: "success",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "build_info",
				},
			},
			mockBuildinfoFunc: func(ctx context.Context) (promv1.BuildinfoResult, error) {
				return promv1.BuildinfoResult{}, nil
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
					Name: "build_info",
				},
			},
			mockBuildinfoFunc: func(ctx context.Context) (promv1.BuildinfoResult, error) {
				return promv1.BuildinfoResult{}, errors.New("prometheus exploded")
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "prometheus exploded")
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(prometheusBuildinfoTool, prometheusBuildinfoToolHandler)

	promApi := promApi{
		API: mockAPI,
	}
	ctx := addApiClientToContext(context.Background(), promApi)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.BuildinfoFunc = tc.mockBuildinfoFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
			tc.validateResult(t, res, err)
		})
	}
}

func TestFlagsToolHandler(t *testing.T) {
	testCases := []struct {
		name           string
		request        mcp.CallToolRequest
		mockFlagsFunc  func(ctx context.Context) (promv1.FlagsResult, error)
		validateResult func(t *testing.T, result *mcp.CallToolResult, err error)
	}{
		{
			name: "success",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "flags",
				},
			},
			mockFlagsFunc: func(ctx context.Context) (promv1.FlagsResult, error) {
				return promv1.FlagsResult{}, nil
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
					Name: "flags",
				},
			},
			mockFlagsFunc: func(ctx context.Context) (promv1.FlagsResult, error) {
				return promv1.FlagsResult{}, errors.New("prometheus exploded")
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "prometheus exploded")
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(prometheusFlagsTool, prometheusFlagsToolHandler)

	promApi := promApi{
		API: mockAPI,
	}
	ctx := addApiClientToContext(context.Background(), promApi)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.FlagsFunc = tc.mockFlagsFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
			tc.validateResult(t, res, err)
		})
	}
}

func TestListAlertsToolHandler(t *testing.T) {
	testCases := []struct {
		name           string
		request        mcp.CallToolRequest
		mockAlertsFunc func(ctx context.Context) (promv1.AlertsResult, error)
		validateResult func(t *testing.T, result *mcp.CallToolResult, err error)
	}{
		{
			name: "success",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "list_alerts",
				},
			},
			mockAlertsFunc: func(ctx context.Context) (promv1.AlertsResult, error) {
				return promv1.AlertsResult{}, nil
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
					Name: "list_alerts",
				},
			},
			mockAlertsFunc: func(ctx context.Context) (promv1.AlertsResult, error) {
				return promv1.AlertsResult{}, errors.New("prometheus exploded")
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "prometheus exploded")
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(prometheusListAlertsTool, prometheusListAlertsToolHandler)

	promApi := promApi{
		API: mockAPI,
	}
	ctx := addApiClientToContext(context.Background(), promApi)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.AlertsFunc = tc.mockAlertsFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
			tc.validateResult(t, res, err)
		})
	}
}

func TestLabelValuesToolHandler(t *testing.T) {
	testCases := []struct {
		name                string
		request             mcp.CallToolRequest
		mockLabelValuesFunc func(ctx context.Context, label string, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) (model.LabelValues, promv1.Warnings, error)
		validateResult      func(t *testing.T, result *mcp.CallToolResult, err error)
	}{
		{
			name: "success",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "label_values",
					Arguments: map[string]any{
						"label":      "__name__",
						"start_time": "1756142748",
						"end_time":   "1756143048",
					},
				},
			},
			mockLabelValuesFunc: func(ctx context.Context, label string, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) (model.LabelValues, promv1.Warnings, error) {
				require.Equal(t, "__name__", label)
				return model.LabelValues{}, nil, nil
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsError)
			},
		},
		{
			name: "missing label",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name:      "label_values",
					Arguments: map[string]any{},
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "label must be a string")
			},
		},
		{
			name: "API error",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name:      "label_values",
					Arguments: map[string]any{"label": "up"},
				},
			},
			mockLabelValuesFunc: func(ctx context.Context, label string, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) (model.LabelValues, promv1.Warnings, error) {
				return nil, nil, errors.New("prometheus exploded")
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "prometheus exploded")
			},
		},
		{
			name: "invalid start_time",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name:      "label_values",
					Arguments: map[string]any{"label": "up", "start_time": "not-a-real-timestamp"},
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "failed to parse start_time")
			},
		},
		{
			name: "invalid end_time",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name:      "label_values",
					Arguments: map[string]any{"label": "up", "end_time": "not-a-real-timestamp"},
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "failed to parse end_time")
			},
		},
		{
			name: "truncation - truncated output with warning",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "label_values",
					Arguments: map[string]any{
						"label":            "__name__",
						"start_time":       "1756142748",
						"end_time":         "1756143048",
						"truncation_limit": 1,
					},
				},
			},
			mockLabelValuesFunc: func(ctx context.Context, label string, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) (model.LabelValues, promv1.Warnings, error) {
				require.Equal(t, "__name__", label)
				mockLabelValues := model.LabelValues{"value1", "value2", "value3"}
				return mockLabelValues, nil, nil
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsError)
				expectedWarning := strings.ReplaceAll(displayTruncationWarning(1), "\n", "\\n")
				expectedResult := fmt.Sprintf(`{"result":"value1%s","warnings":null}`, expectedWarning)
				require.JSONEq(t, expectedResult, getToolCallResultAsString(result))
			},
		},
		{
			name: "truncation - not truncated, no warning",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "label_values",
					Arguments: map[string]any{
						"label":            "__name__",
						"start_time":       "1756142748",
						"end_time":         "1756143048",
						"truncation_limit": 0,
					},
				},
			},
			mockLabelValuesFunc: func(ctx context.Context, label string, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) (model.LabelValues, promv1.Warnings, error) {
				require.Equal(t, "__name__", label)
				mockLabelValues := model.LabelValues{"value1", "value2", "value3"}
				return mockLabelValues, nil, nil
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsError)
				expectedResult := `{"result":"value1\nvalue2\nvalue3","warnings":null}`
				require.JSONEq(t, expectedResult, getToolCallResultAsString(result))
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(prometheusLabelValuesTool, prometheusLabelValuesToolHandler)

	promApi := promApi{
		API: mockAPI,
	}
	ctx := addApiClientToContext(context.Background(), promApi)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.LabelValuesFunc = tc.mockLabelValuesFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
			tc.validateResult(t, res, err)
		})
	}
}

func TestSeriesToolHandler(t *testing.T) {
	testCases := []struct {
		name           string
		request        mcp.CallToolRequest
		mockSeriesFunc func(ctx context.Context, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) ([]model.LabelSet, promv1.Warnings, error)
		validateResult func(t *testing.T, result *mcp.CallToolResult, err error)
	}{
		{
			name: "success",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "series",
					Arguments: map[string]any{
						"matches":    []string{"up"},
						"start_time": "1756142748",
						"end_time":   "1756143048",
					},
				},
			},
			mockSeriesFunc: func(ctx context.Context, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) ([]model.LabelSet, promv1.Warnings, error) {
				require.Equal(t, []string{"up"}, matches)
				return []model.LabelSet{}, nil, nil
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
					Name:      "series",
					Arguments: map[string]any{},
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "matches must be an array")
			},
		},
		{
			name: "API error",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name:      "series",
					Arguments: map[string]any{"matches": []string{"up"}},
				},
			},
			mockSeriesFunc: func(ctx context.Context, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) ([]model.LabelSet, promv1.Warnings, error) {
				return nil, nil, errors.New("prometheus exploded")
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "prometheus exploded")
			},
		},
		{
			name: "invalid start_time",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name:      "series",
					Arguments: map[string]any{"matches": []string{"up"}, "start_time": "not-a-real-timestamp"},
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "failed to parse start_time")
			},
		},
		{
			name: "invalid end_time",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name:      "series",
					Arguments: map[string]any{"matches": []string{"up"}, "end_time": "not-a-real-timestamp"},
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "failed to parse end_time")
			},
		},
		{
			name: "truncation - truncated output with warning",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "series",
					Arguments: map[string]any{
						"matches":          []string{"up"},
						"start_time":       "1756142748",
						"end_time":         "1756143048",
						"truncation_limit": 1,
					},
				},
			},
			mockSeriesFunc: func(ctx context.Context, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) ([]model.LabelSet, promv1.Warnings, error) {
				require.Equal(t, []string{"up"}, matches)
				mockLabelSets := []model.LabelSet{
					{"__name__": "up", "instance": "localhost:9090", "job": "prometheus"},
					{"__name__": "up", "instance": "localhost:9100", "job": "node-exporter"},
				}
				return mockLabelSets, nil, nil
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsError)
				expectedWarning := strings.ReplaceAll(displayTruncationWarning(1), "\n", "\\n")
				expectedResult := fmt.Sprintf(`{"result":"{__name__=\u0022up\u0022, instance=\u0022localhost:9090\u0022, job=\u0022prometheus\u0022}%s","warnings":null}`, expectedWarning)
				require.JSONEq(t, expectedResult, getToolCallResultAsString(result))
			},
		},
		{
			name: "truncation - not truncated, no warning",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "series",
					Arguments: map[string]any{
						"matches":          []string{"up"},
						"start_time":       "1756142748",
						"end_time":         "1756143048",
						"truncation_limit": 0,
					},
				},
			},
			mockSeriesFunc: func(ctx context.Context, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) ([]model.LabelSet, promv1.Warnings, error) {
				require.Equal(t, []string{"up"}, matches)
				mockLabelSets := []model.LabelSet{
					{"__name__": "up", "instance": "localhost:9090", "job": "prometheus"},
					{"__name__": "up", "instance": "localhost:9100", "job": "node-exporter"},
				}
				return mockLabelSets, nil, nil
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsError)
				expectedResult := `{"result":"{__name__=\u0022up\u0022, instance=\u0022localhost:9090\u0022, job=\u0022prometheus\u0022}\n{__name__=\u0022up\u0022, instance=\u0022localhost:9100\u0022, job=\u0022node-exporter\u0022}","warnings":null}`
				require.JSONEq(t, expectedResult, getToolCallResultAsString(result))
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddTool(prometheusSeriesTool, prometheusSeriesToolHandler)

	promApi := promApi{
		API: mockAPI,
	}
	ctx := addApiClientToContext(context.Background(), promApi)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.SeriesFunc = tc.mockSeriesFunc

			res, err := mcpClient.CallTool(ctx, tc.request)
			tc.validateResult(t, res, err)
		})
	}
}

func TestPrometheusManagementAPITools(t *testing.T) {
	testCases := []struct {
		name                 string
		tool                 mcp.Tool
		method               string
		path                 string
		handler              server.ToolHandlerFunc
		request              mcp.CallToolRequest
		mockRoundTripper     *mockRoundTripper
		validateResult       func(t *testing.T, result *mcp.CallToolResult, err error)
		expectApiClientError bool
	}{
		// Healthy tool tests
		{
			name:    "healthy success",
			tool:    prometheusHealthyTool,
			handler: prometheusHealthyToolHandler,
			method:  http.MethodGet,
			path:    mgmtApiHealthyEndpoint,
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params:  mcp.CallToolParams{Name: "healthy"},
			},
			mockRoundTripper: &mockRoundTripper{
				Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString("Prometheus is Healthy.\n")),
					Header:     make(http.Header),
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "Prometheus is Healthy.")
			},
			expectApiClientError: false,
		},
		{
			name:    "healthy API error - non-200 status code",
			tool:    prometheusHealthyTool,
			handler: prometheusHealthyToolHandler,
			method:  http.MethodGet,
			path:    mgmtApiHealthyEndpoint,
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params:  mcp.CallToolParams{Name: "healthy"},
			},
			mockRoundTripper: &mockRoundTripper{
				Response: &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(bytes.NewBufferString("Internal Server Error")),
					Header:     make(http.Header),
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "received non-ok HTTP status code")
			},
			expectApiClientError: false,
		},
		{
			name:    "healthy API error - roundtripper error",
			tool:    prometheusHealthyTool,
			handler: prometheusHealthyToolHandler,
			method:  http.MethodGet,
			path:    mgmtApiHealthyEndpoint,
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params:  mcp.CallToolParams{Name: "healthy"},
			},
			mockRoundTripper: &mockRoundTripper{
				Error: errors.New("network unreachable"),
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "network unreachable")
			},
			expectApiClientError: false,
		},
		{
			name:    "healthy no client in context",
			tool:    prometheusHealthyTool,
			handler: prometheusHealthyToolHandler,
			method:  http.MethodGet,
			path:    mgmtApiHealthyEndpoint,
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params:  mcp.CallToolParams{Name: "healthy"},
			},
			mockRoundTripper: nil,
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "error getting prometheus api client from context")
			},
			expectApiClientError: true,
		},

		// Ready tool tests
		{
			name:    "ready success",
			tool:    prometheusReadyTool,
			handler: prometheusReadyToolHandler,
			method:  http.MethodGet,
			path:    mgmtApiReadyEndpoint,
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params:  mcp.CallToolParams{Name: "ready"},
			},
			mockRoundTripper: &mockRoundTripper{
				Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString("Prometheus is Ready.\n")),
					Header:     make(http.Header),
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "Prometheus is Ready.")
			},
			expectApiClientError: false,
		},
		{
			name:    "ready API error - non-200 status code",
			tool:    prometheusReadyTool,
			handler: prometheusReadyToolHandler,
			method:  http.MethodGet,
			path:    mgmtApiReadyEndpoint,
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params:  mcp.CallToolParams{Name: "ready"},
			},
			mockRoundTripper: &mockRoundTripper{
				Response: &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(bytes.NewBufferString("Internal Server Error")),
					Header:     make(http.Header),
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "received non-ok HTTP status code")
			},
			expectApiClientError: false,
		},
		{
			name:    "ready API error - roundtripper error",
			tool:    prometheusReadyTool,
			handler: prometheusReadyToolHandler,
			method:  http.MethodGet,
			path:    mgmtApiReadyEndpoint,
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params:  mcp.CallToolParams{Name: "ready"},
			},
			mockRoundTripper: &mockRoundTripper{
				Error: errors.New("network unreachable"),
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "network unreachable")
			},
			expectApiClientError: false,
		},
		{
			name:    "ready no client in context",
			tool:    prometheusReadyTool,
			handler: prometheusReadyToolHandler,
			method:  http.MethodGet,
			path:    mgmtApiReadyEndpoint,
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params:  mcp.CallToolParams{Name: "ready"},
			},
			mockRoundTripper: nil,
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "error getting prometheus api client from context")
			},
			expectApiClientError: true,
		},

		// Reload tool tests
		{
			name:    "reload success",
			tool:    prometheusReloadTool,
			handler: prometheusReloadToolHandler,
			method:  http.MethodPost,
			path:    mgmtApiReloadEndpoint,
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params:  mcp.CallToolParams{Name: "reload"},
			},
			mockRoundTripper: &mockRoundTripper{
				Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString("Configuration reloaded.\n")),
					Header:     make(http.Header),
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "Configuration reloaded.")
			},
			expectApiClientError: false,
		},
		{
			name:    "reload API error - non-200 status code",
			tool:    prometheusReloadTool,
			handler: prometheusReloadToolHandler,
			method:  http.MethodPost,
			path:    mgmtApiReloadEndpoint,
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params:  mcp.CallToolParams{Name: "reload"},
			},
			mockRoundTripper: &mockRoundTripper{
				Response: &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(bytes.NewBufferString("Internal Server Error")),
					Header:     make(http.Header),
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "received non-ok HTTP status code")
			},
			expectApiClientError: false,
		},
		{
			name:    "reload API error - roundtripper error",
			tool:    prometheusReloadTool,
			handler: prometheusReloadToolHandler,
			method:  http.MethodPost,
			path:    mgmtApiReloadEndpoint,
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params:  mcp.CallToolParams{Name: "reload"},
			},
			mockRoundTripper: &mockRoundTripper{
				Error: errors.New("network unreachable"),
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "network unreachable")
			},
			expectApiClientError: false,
		},
		{
			name:    "reload no client in context",
			tool:    prometheusReloadTool,
			handler: prometheusReloadToolHandler,
			method:  http.MethodPost,
			path:    mgmtApiReloadEndpoint,
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params:  mcp.CallToolParams{Name: "reload"},
			},
			mockRoundTripper: nil,
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "error getting prometheus api client from context")
			},
			expectApiClientError: true,
		},

		// Quit tool tests
		{
			name:    "quit success",
			tool:    prometheusQuitTool,
			handler: prometheusQuitToolHandler,
			method:  http.MethodPost,
			path:    mgmtApiQuitEndpoint,
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params:  mcp.CallToolParams{Name: "quit"},
			},
			mockRoundTripper: &mockRoundTripper{
				Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString("Shutting down.\n")),
					Header:     make(http.Header),
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "Shutting down.")
			},
			expectApiClientError: false,
		},
		{
			name:    "quit API error - non-200 status code",
			tool:    prometheusQuitTool,
			handler: prometheusQuitToolHandler,
			method:  http.MethodPost,
			path:    mgmtApiQuitEndpoint,
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params:  mcp.CallToolParams{Name: "quit"},
			},
			mockRoundTripper: &mockRoundTripper{
				Response: &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(bytes.NewBufferString("Internal Server Error")),
					Header:     make(http.Header),
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "received non-ok HTTP status code")
			},
			expectApiClientError: false,
		},
		{
			name:    "quit API error - roundtripper error",
			tool:    prometheusQuitTool,
			handler: prometheusQuitToolHandler,
			method:  http.MethodPost,
			path:    mgmtApiQuitEndpoint,
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params:  mcp.CallToolParams{Name: "quit"},
			},
			mockRoundTripper: &mockRoundTripper{
				Error: errors.New("network unreachable"),
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "network unreachable")
			},
			expectApiClientError: false,
		},
		{
			name:    "quit no client in context",
			tool:    prometheusQuitTool,
			handler: prometheusQuitToolHandler,
			method:  http.MethodPost,
			path:    mgmtApiQuitEndpoint,
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params:  mcp.CallToolParams{Name: "quit"},
			},
			mockRoundTripper: nil,
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Contains(t, getToolCallResultAsString(result), "error getting prometheus api client from context")
			},
			expectApiClientError: true,
		},
	}

	// This is needed to satisfy the promv1.API interface that promApi embeds.
	mockAPI := &MockPrometheusAPI{}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockServer := mcptest.NewUnstartedServer(t)
			mockServer.AddTool(tc.tool, tc.handler)

			ctx := context.Background()
			if !tc.expectApiClientError {
				p := promApi{
					API:          mockAPI,
					url:          "http://localhost:9090",
					roundtripper: http.DefaultTransport,
				}

				if tc.mockRoundTripper != nil {
					p.roundtripper = tc.mockRoundTripper
				}
				ctx = addApiClientToContext(ctx, p)
			}

			err := mockServer.Start(ctx)
			require.NoError(t, err)
			defer mockServer.Close()

			mcpClient := mockServer.Client()
			defer mcpClient.Close()

			res, err := mcpClient.CallTool(ctx, tc.request)
			tc.validateResult(t, res, err)
		})
	}
}
