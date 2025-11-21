package mcp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/stretchr/testify/require"
)

// mockRoundTripper is a mock implementation of http.RoundTripper.
type mockRoundTripper struct {
	Response *http.Response
	Error    error
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return m.Response, nil
}

func TestThanosStoresToolHandler(t *testing.T) {
	storesJson := `{"status":"success","data":{"sidecar":[{"name":"127.0.0.1:19091","lastCheck":"2025-11-20T19:23:34.434377663-05:00","lastError":null,"labelSets":[{"host":"test"}],"minTime":1756088522000,"maxTime":9223372036854775807}]}}`
	testCases := []struct {
		name                 string
		request              mcp.CallToolRequest
		mockRoundTripper     *mockRoundTripper
		validateResult       func(t *testing.T, result *mcp.CallToolResult, err error)
		expectApiClientError bool
	}{
		{
			name: "success",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "list_stores",
				},
			},
			mockRoundTripper: &mockRoundTripper{
				Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(storesJson)),

					Header: make(http.Header),
				},
			},
			validateResult: func(t *testing.T, result *mcp.CallToolResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsError)
				require.JSONEq(t, storesJson, getToolCallResultAsString(result))
			},
			expectApiClientError: false,
		},
		{
			name: "API error - non-200 status code",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "list_stores",
				},
			},
			mockRoundTripper: &mockRoundTripper{
				Response: &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(bytes.NewBufferString(`{"error":"internal server error"}`)),
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
			name: "API error - roundtripper error",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "list_stores",
				},
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
			name: "no client in context",
			request: mcp.CallToolRequest{
				Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
				Params: mcp.CallToolParams{
					Name: "list_stores",
				},
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
	// For thanos_tools, we are mocking the http.RoundTripper directly for doHttpRequest.
	// So, the actual Prometheus API calls (like Query, QueryRange, etc.) will not be made.
	// Therefore, we can use a basic MockPrometheusAPI here.
	mockAPI := &MockPrometheusAPI{}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockServer := mcptest.NewUnstartedServer(t)
			mockServer.AddTool(thanosStoresTool, thanosStoresToolHandler)

			ctx := context.Background()
			if !tc.expectApiClientError {
				p := promApi{
					API:          mockAPI,
					url:          "http://thanos-query:10902",
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
