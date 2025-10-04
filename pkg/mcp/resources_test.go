package mcp

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestListMetricsResourceHandler(t *testing.T) {
	testCases := []struct {
		name                string
		request             mcp.ReadResourceRequest
		mockLabelValuesFunc func(ctx context.Context, label string, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) (model.LabelValues, promv1.Warnings, error)
		validateResult      func(t *testing.T, res *mcp.ReadResourceResult, err error)
	}{
		{
			name: "Success",
			request: mcp.ReadResourceRequest{
				Params: mcp.ReadResourceParams{
					URI: resourcePrefix + "list_metrics",
				},
			},
			mockLabelValuesFunc: func(ctx context.Context, label string, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) (model.LabelValues, promv1.Warnings, error) {
				return model.LabelValues{"metric1", "metric2"}, nil, nil
			},
			validateResult: func(t *testing.T, res *mcp.ReadResourceResult, err error) {
				require.NoError(t, err)
				require.Len(t, res.Contents, 1)
				require.JSONEq(t, "{\"result\":\"metric1\\nmetric2\",\"warnings\":null}", getTextResourceContentsAsString(res.Contents))
			},
		},
		{
			name: "API Error",
			request: mcp.ReadResourceRequest{
				Params: mcp.ReadResourceParams{
					URI: resourcePrefix + "list_metrics",
				},
			},
			mockLabelValuesFunc: func(ctx context.Context, label string, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) (model.LabelValues, promv1.Warnings, error) {
				return nil, nil, errors.New("api error")
			},
			validateResult: func(t *testing.T, res *mcp.ReadResourceResult, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "api error")
			},
		},
		{
			name: "Empty Metric List",
			request: mcp.ReadResourceRequest{
				Params: mcp.ReadResourceParams{
					URI: resourcePrefix + "list_metrics",
				},
			},
			mockLabelValuesFunc: func(ctx context.Context, label string, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) (model.LabelValues, promv1.Warnings, error) {
				return model.LabelValues{}, nil, nil
			},
			validateResult: func(t *testing.T, res *mcp.ReadResourceResult, err error) {
				require.NoError(t, err)
				require.Len(t, res.Contents, 1)
				require.JSONEq(t, "{\"result\":\"\",\"warnings\":null}", getTextResourceContentsAsString(res.Contents))
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddResource(listMetricsResource, listMetricsResourceHandler)

	ctx := addApiClientToContext(context.Background(), mockAPI)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.LabelValuesFunc = tc.mockLabelValuesFunc

			res, err := mcpClient.ReadResource(ctx, tc.request)
			fmt.Printf("TJ DEBUG | test name: %s | res: %+v | err: %s", tc.name, res, err)
			tc.validateResult(t, res, err)
		})
	}
}

func TestTargetsResourceHandler(t *testing.T) {
	testCases := []struct {
		name            string
		request         mcp.ReadResourceRequest
		mockTargetsFunc func(ctx context.Context) (promv1.TargetsResult, error)
		validateResult  func(t *testing.T, res *mcp.ReadResourceResult, err error)
	}{
		{
			name: "Success",
			request: mcp.ReadResourceRequest{
				Params: mcp.ReadResourceParams{
					URI: resourcePrefix + "targets",
				},
			},
			mockTargetsFunc: func(ctx context.Context) (promv1.TargetsResult, error) {
				return promv1.TargetsResult{
					Active: []promv1.ActiveTarget{
						{
							DiscoveredLabels: map[string]string{"__address__": "localhost:9090"},
							Labels:           model.LabelSet{"job": "prometheus"},
							ScrapeURL:        "http://localhost:9090/metrics",
							Health:           promv1.HealthGood,
						},
					},
				}, nil
			},
			validateResult: func(t *testing.T, res *mcp.ReadResourceResult, err error) {
				require.NoError(t, err)
				require.Len(t, res.Contents, 1)
				require.Contains(t, getTextResourceContentsAsString(res.Contents), "localhost:9090")
				require.Contains(t, getTextResourceContentsAsString(res.Contents), "prometheus")
			},
		},
		{
			name: "API Error",
			request: mcp.ReadResourceRequest{
				Params: mcp.ReadResourceParams{
					URI: resourcePrefix + "targets",
				},
			},
			mockTargetsFunc: func(ctx context.Context) (promv1.TargetsResult, error) {
				return promv1.TargetsResult{}, errors.New("api error")
			},
			validateResult: func(t *testing.T, res *mcp.ReadResourceResult, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "api error")
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddResource(targetsResource, targetsResourceHandler)

	ctx := addApiClientToContext(context.Background(), mockAPI)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.TargetsFunc = tc.mockTargetsFunc

			res, err := mcpClient.ReadResource(ctx, tc.request)
			tc.validateResult(t, res, err)
		})
	}
}

func TestTsdbStatsResourceHandler(t *testing.T) {
	testCases := []struct {
		name           string
		request        mcp.ReadResourceRequest
		mockTSDBFunc   func(ctx context.Context, opts ...promv1.Option) (promv1.TSDBResult, error)
		validateResult func(t *testing.T, res *mcp.ReadResourceResult, err error)
	}{
		{
			name: "Success",
			request: mcp.ReadResourceRequest{
				Params: mcp.ReadResourceParams{
					URI: resourcePrefix + "tsdb_stats",
				},
			},
			mockTSDBFunc: func(ctx context.Context, opts ...promv1.Option) (promv1.TSDBResult, error) {
				return promv1.TSDBResult{
					SeriesCountByMetricName: []promv1.Stat{{Name: "up", Value: 1}},
				}, nil
			},
			validateResult: func(t *testing.T, res *mcp.ReadResourceResult, err error) {
				require.NoError(t, err)
				require.Len(t, res.Contents, 1)
				require.Contains(t, getTextResourceContentsAsString(res.Contents), "up")
			},
		},
		{
			name: "API Error",
			request: mcp.ReadResourceRequest{
				Params: mcp.ReadResourceParams{
					URI: resourcePrefix + "tsdb_stats",
				},
			},
			mockTSDBFunc: func(ctx context.Context, opts ...promv1.Option) (promv1.TSDBResult, error) {
				return promv1.TSDBResult{}, errors.New("api error")
			},
			validateResult: func(t *testing.T, res *mcp.ReadResourceResult, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "api error")
			},
		},
	}

	mockAPI := &MockPrometheusAPI{}
	mockServer := mcptest.NewUnstartedServer(t)
	mockServer.AddResource(tsdbStatsResource, tsdbStatsResourceHandler)

	ctx := addApiClientToContext(context.Background(), mockAPI)
	err := mockServer.Start(ctx)
	require.NoError(t, err)
	defer mockServer.Close()

	mcpClient := mockServer.Client()
	defer mcpClient.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI.TSDBFunc = tc.mockTSDBFunc

			res, err := mcpClient.ReadResource(ctx, tc.request)
			tc.validateResult(t, res, err)
		})
	}
}

// createMockDocsFS creates a mock filesystem for testing documentation resources.
func createMockDocsFS() fs.FS {
	return fstest.MapFS{
		"doc1.md": {
			Data: []byte("This is the first test document."),
		},
		"folder/doc2.md": {
			Data: []byte("This is the second test document in a folder."),
		},
	}
}

func TestDocsListResourceHandler(t *testing.T) {
	testCases := []struct {
		name           string
		request        mcp.ReadResourceRequest
		mockDocsFS     fs.FS
		validateResult func(t *testing.T, res *mcp.ReadResourceResult, err error)
	}{
		{
			name: "Success",
			request: mcp.ReadResourceRequest{
				Params: mcp.ReadResourceParams{
					URI: resourcePrefix + "docs",
				},
			},
			mockDocsFS: createMockDocsFS(),
			validateResult: func(t *testing.T, res *mcp.ReadResourceResult, err error) {
				require.NoError(t, err)
				require.Len(t, res.Contents, 1)

				actual := getTextResourceContentsAsString(res.Contents)
				parts := strings.Split(actual, "\n")
				// fstest.MapFS does not guarantee order.
				sort.Strings(parts)
				require.Len(t, parts, 2)
				require.Equal(t, "doc1.md", parts[0])
				require.Equal(t, "folder/doc2.md", parts[1])
			},
		},
		{
			name:       "Empty Filesystem",
			mockDocsFS: fstest.MapFS{},
			validateResult: func(t *testing.T, res *mcp.ReadResourceResult, err error) {
				require.Error(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockServer := mcptest.NewUnstartedServer(t)
			mockServer.AddResource(docsListResource, docsListResourceHandler)

			ctx := addDocsToContext(context.Background(), tc.mockDocsFS)
			err := mockServer.Start(ctx)
			require.NoError(t, err)
			defer mockServer.Close()

			mcpClient := mockServer.Client()
			defer mcpClient.Close()

			res, err := mcpClient.ReadResource(ctx, tc.request)
			tc.validateResult(t, res, err)
		})
	}
}

func TestDocsReadResourceTemplateHandler(t *testing.T) {
	testCases := []struct {
		name           string
		request        mcp.ReadResourceRequest
		mockDocsFS     fs.FS
		validateResult func(t *testing.T, res *mcp.ReadResourceResult, err error)
	}{
		{
			name: "Success",
			request: mcp.ReadResourceRequest{
				Params: mcp.ReadResourceParams{
					URI: resourcePrefix + "docs/doc1.md",
				},
			},
			mockDocsFS: createMockDocsFS(),
			validateResult: func(t *testing.T, res *mcp.ReadResourceResult, err error) {
				require.NoError(t, err)
				require.Len(t, res.Contents, 1)
				require.Equal(t, "This is the first test document.", getTextResourceContentsAsString(res.Contents))
			},
		},
		{
			name: "File Not Found",
			request: mcp.ReadResourceRequest{
				Params: mcp.ReadResourceParams{
					URI: resourcePrefix + "docs/non_existent_file.md",
				},
			},
			mockDocsFS: createMockDocsFS(),
			validateResult: func(t *testing.T, res *mcp.ReadResourceResult, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "file does not exist")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockServer := mcptest.NewUnstartedServer(t)
			mockServer.AddResourceTemplate(docsReadResourceTemplate, docsReadResourceTemplateHandler)

			ctx := addDocsToContext(context.Background(), tc.mockDocsFS)
			err := mockServer.Start(ctx)
			require.NoError(t, err)
			defer mockServer.Close()

			mcpClient := mockServer.Client()
			defer mcpClient.Close()

			res, err := mcpClient.ReadResource(ctx, tc.request)
			tc.validateResult(t, res, err)
		})
	}
}
