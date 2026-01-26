package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// newTestLogger creates a slog.Logger backed by a bytes.Buffer for log
// assertion. The logger writes JSON-formatted entries at LevelDebug so all
// log levels are captured.
func newTestLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return logger, &buf
}

// mockRequest constructs a ServerRequest suitable for middleware tests. The
// Session field is nil because the telemetry middleware never accesses it.
func mockRequest[P mcp.Params](params P) mcp.Request {
	return &mcp.ServerRequest[P]{Params: params}
}

func TestTelemetryMiddleware_Routing(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		method         string
		req            mcp.Request
		nextResult     mcp.Result
		nextErr        error
		wantLogged     string // substring expected in log output; empty means no specific check
		wantNotLogged  string // substring that must NOT appear; empty means no check
		expectNextCall bool
	}{
		{
			name:   "unknown method passes through without instrumentation",
			method: "some/unknown",
			req:    mockRequest(&mcp.PingParams{}),
			nextResult: &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "passthrough"}},
			},
			nextErr:        nil,
			wantNotLogged:  "Calling tool",
			expectNextCall: true,
		},
		{
			name:   "initialize method dispatches to initialize handler",
			method: methodInitialize,
			req: mockRequest(&mcp.InitializeParams{
				ProtocolVersion: "2025-03-26",
				ClientInfo:      &mcp.Implementation{Name: "test-client", Version: "0.1"},
				Capabilities:    &mcp.ClientCapabilities{},
			}),
			nextResult: &mcp.InitializeResult{
				ProtocolVersion: "2025-03-26",
				ServerInfo:      &mcp.Implementation{Name: "test-server", Version: "0.2"},
			},
			nextErr:        nil,
			wantLogged:     "MCP server initialized",
			expectNextCall: true,
		},
		{
			name:   "tools/call method dispatches to tool call handler",
			method: methodToolsCall,
			req: mockRequest(&mcp.CallToolParamsRaw{
				Name:      "query",
				Arguments: json.RawMessage(`{"query":"up"}`),
			}),
			nextResult: &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
			},
			nextErr:        nil,
			wantLogged:     "Calling tool",
			expectNextCall: true,
		},
		{
			name:   "resources/read method dispatches to resource read handler",
			method: methodResourcesRead,
			req: mockRequest(&mcp.ReadResourceParams{
				URI: "prometheus://metrics",
			}),
			nextResult: &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{URI: "prometheus://metrics", Text: "data"}},
			},
			nextErr:        nil,
			wantLogged:     "Calling resource",
			expectNextCall: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			logger, buf := newTestLogger()
			middleware := telemetryMiddleware(logger)

			nextCalled := false
			next := func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				nextCalled = true
				return tc.nextResult, tc.nextErr
			}

			handler := middleware(next)
			result, err := handler(context.Background(), tc.method, tc.req)

			require.Equal(t, tc.expectNextCall, nextCalled, "next handler call expectation mismatch")

			if tc.nextErr != nil {
				require.ErrorIs(t, err, tc.nextErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.nextResult, result)

			output := buf.String()
			if tc.wantLogged != "" {
				require.Contains(t, output, tc.wantLogged)
			}
			if tc.wantNotLogged != "" {
				require.NotContains(t, output, tc.wantNotLogged)
			}
		})
	}
}

func TestTelemetryHandleInitialize(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		req           mcp.Request
		nextResult    mcp.Result
		nextErr       error
		wantLogged    string   // substring expected in log output
		wantLogFields []string // additional substrings expected in log output (structured field values)
		wantErr       bool
	}{
		{
			name: "successful initialization logs client and server info",
			req: mockRequest(&mcp.InitializeParams{
				ProtocolVersion: "2025-03-26",
				ClientInfo:      &mcp.Implementation{Name: "test-client", Version: "1.0"},
				Capabilities:    &mcp.ClientCapabilities{},
			}),
			nextResult: &mcp.InitializeResult{
				ProtocolVersion: "2025-03-26",
				ServerInfo:      &mcp.Implementation{Name: "test-server", Version: "2.0"},
			},
			nextErr:       nil,
			wantLogged:    "MCP server initialized",
			wantLogFields: []string{"test-client", "test-server"},
			wantErr:       false,
		},
		{
			name: "successful initialization with nil client info",
			req: mockRequest(&mcp.InitializeParams{
				ProtocolVersion: "2025-03-26",
				Capabilities:    &mcp.ClientCapabilities{},
			}),
			nextResult: &mcp.InitializeResult{
				ProtocolVersion: "2025-03-26",
				ServerInfo:      &mcp.Implementation{Name: "srv", Version: "1"},
			},
			nextErr:    nil,
			wantLogged: "MCP server initialized",
			wantErr:    false,
		},
		{
			name: "successful initialization with nil server info does not panic",
			req: mockRequest(&mcp.InitializeParams{
				ProtocolVersion: "2025-03-26",
				ClientInfo:      &mcp.Implementation{Name: "test-client", Version: "1.0"},
				Capabilities:    &mcp.ClientCapabilities{},
			}),
			nextResult: &mcp.InitializeResult{
				ProtocolVersion: "2025-03-26",
				ServerInfo:      nil,
			},
			nextErr:    nil,
			wantLogged: "MCP server initialized",
			wantErr:    false,
		},
		{
			name: "failed initialization logs error",
			req: mockRequest(&mcp.InitializeParams{
				ProtocolVersion: "2025-03-26",
				ClientInfo:      &mcp.Implementation{Name: "c", Version: "1"},
				Capabilities:    &mcp.ClientCapabilities{},
			}),
			nextResult: nil,
			nextErr:    errors.New("init boom"),
			wantLogged: "MCP initialization failed",
			wantErr:    true,
		},
		{
			name:       "invalid params type falls through gracefully",
			req:        mockRequest(&mcp.PingParams{}),
			nextResult: &mcp.InitializeResult{ProtocolVersion: "2025-03-26"},
			nextErr:    nil,
			wantLogged: "Failed to extract initialize params for telemetry",
			wantErr:    false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			logger, buf := newTestLogger()

			nextCalled := false
			next := func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				nextCalled = true
				return tc.nextResult, tc.nextErr
			}

			result, err := telemetryHandleInitialize(context.Background(), methodInitialize, tc.req, next, logger)

			require.True(t, nextCalled, "expected next handler to be called")

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// The handler always returns whatever next returns (possibly nil).
			require.Equal(t, tc.nextResult, result)

			output := buf.String()
			require.Contains(t, output, tc.wantLogged)
			for _, field := range tc.wantLogFields {
				require.Contains(t, output, field, "expected structured log field value %q in output", field)
			}
		})
	}
}

func TestTelemetryHandleToolCall(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		req           mcp.Request
		nextResult    mcp.Result
		nextErr       error
		wantLogged    string
		wantLogFields []string // additional substrings expected in log output (structured field values)
		wantErr       bool
	}{
		{
			name: "successful tool call logs tool name and duration",
			req: mockRequest(&mcp.CallToolParamsRaw{
				Name:      "query",
				Arguments: json.RawMessage(`{"query":"up"}`),
			}),
			nextResult: &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "result data"}},
			},
			nextErr:       nil,
			wantLogged:    "Finished calling tool",
			wantLogFields: []string{"query", "duration"},
			wantErr:       false,
		},
		{
			name: "failed tool call logs error",
			req: mockRequest(&mcp.CallToolParamsRaw{
				Name:      "query",
				Arguments: json.RawMessage(`{"query":"up"}`),
			}),
			nextResult: &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "error"}},
			},
			nextErr:    errors.New("tool boom"),
			wantLogged: "Failed calling tool",
			wantErr:    true,
		},
		{
			name: "failed type assertion on result returns early without panic",
			req: mockRequest(&mcp.CallToolParamsRaw{
				Name:      "query",
				Arguments: json.RawMessage(`{}`),
			}),
			// Return a non-CallToolResult to trigger failed type assertion.
			nextResult: &mcp.InitializeResult{ProtocolVersion: "2025-03-26"},
			nextErr:    nil,
			wantLogged: "Failed to convert result to call tool result",
			wantErr:    false,
		},
		{
			name: "nil result from next returns early without panic",
			req: mockRequest(&mcp.CallToolParamsRaw{
				Name:      "query",
				Arguments: json.RawMessage(`{}`),
			}),
			// Return nil result with an error to trigger failed type
			// assertion on nil interface value.
			nextResult: nil,
			nextErr:    errors.New("nil result boom"),
			wantLogged: "Failed to convert result to call tool result",
			wantErr:    true,
		},
		{
			name: "tool result with IsError true logs failure",
			req: mockRequest(&mcp.CallToolParamsRaw{
				Name:      "query",
				Arguments: json.RawMessage(`{"query":"bad"}`),
			}),
			nextResult: &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "something went wrong"}},
				IsError: true,
			},
			nextErr:    nil,
			wantLogged: "Failed calling tool",
			wantErr:    false,
		},
		{
			name:       "invalid params type falls through gracefully",
			req:        mockRequest(&mcp.PingParams{}),
			nextResult: &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}},
			nextErr:    nil,
			wantLogged: "Failed to extract tool params for telemetry",
			wantErr:    false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			logger, buf := newTestLogger()

			nextCalled := false
			next := func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				nextCalled = true
				return tc.nextResult, tc.nextErr
			}

			result, err := telemetryHandleToolCall(context.Background(), methodToolsCall, tc.req, next, logger)

			require.True(t, nextCalled, "expected next handler to be called")

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.nextResult, result)

			output := buf.String()
			require.Contains(t, output, tc.wantLogged)
			for _, field := range tc.wantLogFields {
				require.Contains(t, output, field, "expected structured log field value %q in output", field)
			}
		})
	}
}

func TestTelemetryHandleResourceRead(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		req           mcp.Request
		nextResult    mcp.Result
		nextErr       error
		wantLogged    string
		wantLogFields []string // additional substrings expected in log output (structured field values)
		wantErr       bool
	}{
		{
			name: "successful resource read logs URI and duration",
			req: mockRequest(&mcp.ReadResourceParams{
				URI: "prometheus://targets",
			}),
			nextResult: &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{URI: "prometheus://targets", Text: "targets data"}},
			},
			nextErr:       nil,
			wantLogged:    "Finished calling resource",
			wantLogFields: []string{"prometheus://targets", "duration"},
			wantErr:       false,
		},
		{
			name: "failed resource read logs error",
			req: mockRequest(&mcp.ReadResourceParams{
				URI: "prometheus://metrics",
			}),
			nextResult: nil,
			nextErr:    errors.New("resource boom"),
			wantLogged: "Failed calling resource",
			wantErr:    true,
		},
		{
			name:       "invalid params type falls through gracefully",
			req:        mockRequest(&mcp.PingParams{}),
			nextResult: &mcp.ReadResourceResult{},
			nextErr:    nil,
			wantLogged: "Failed to extract resource params for telemetry",
			wantErr:    false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			logger, buf := newTestLogger()

			nextCalled := false
			next := func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				nextCalled = true
				return tc.nextResult, tc.nextErr
			}

			result, err := telemetryHandleResourceRead(context.Background(), methodResourcesRead, tc.req, next, logger)

			require.True(t, nextCalled, "expected next handler to be called")

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.nextResult, result)

			output := buf.String()
			require.Contains(t, output, tc.wantLogged)
			for _, field := range tc.wantLogFields {
				require.Contains(t, output, field, "expected structured log field value %q in output", field)
			}
		})
	}
}
