package mcp

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestGetClientLogger(t *testing.T) {
	t.Run("returns nil when request is nil", func(t *testing.T) {
		logger := getClientLogger(nil, "test")
		require.Nil(t, logger, "expected nil logger when request is nil")
	})

	t.Run("returns nil when session is unavailable", func(t *testing.T) {
		// Create a request without a session (GetSession will return nil).
		// CallToolRequest is ServerRequest[*CallToolParamsRaw].
		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Name: "test-tool",
			},
		}

		logger := getClientLogger(req, "test")
		require.Nil(t, logger, "expected nil logger when session is unavailable")
	})

	// Note: Testing the case where session has SendNotification capability
	// requires a full MCP server/client setup with an active session that
	// supports notifications. This is covered implicitly by integration tests
	// via the mcptest harness, which creates a proper client/server session.
	// The getClientLogger function creates an MCP logging handler that sends
	// log messages to connected clients as protocol notifications.
}

func TestGetChainedLogger(t *testing.T) {
	t.Run("returns nop logger when both loggers are nil", func(t *testing.T) {
		logger := getChainedLogger(nil, nil, "test")
		require.NotNil(t, logger, "expected non-nil logger even when both inputs are nil")

		// Verify logging doesn't panic (nop logger should silently discard).
		require.NotPanics(t, func() {
			logger.Info("test message")
		})
	})

	t.Run("returns app logger when request is nil", func(t *testing.T) {
		// Create a test logger that writes to a buffer so we can verify output.
		var buf bytes.Buffer
		appLogger := slog.New(slog.NewTextHandler(&buf, nil))

		logger := getChainedLogger(appLogger, nil, "test")
		require.NotNil(t, logger)

		// Log a message and verify it was captured.
		logger.Info("test message from app logger")
		output := buf.String()
		require.Contains(t, output, "test message from app logger",
			"expected log message to be captured by the app logger")
	})

	t.Run("returns app logger when session is unavailable", func(t *testing.T) {
		// Create a test logger that writes to a buffer.
		var buf bytes.Buffer
		appLogger := slog.New(slog.NewTextHandler(&buf, nil))

		// CallToolRequest is ServerRequest[*CallToolParamsRaw].
		req := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Name: "test-tool",
			},
		}

		logger := getChainedLogger(appLogger, req, "test")
		require.NotNil(t, logger)

		// Log a message and verify it was captured by the app logger.
		logger.Info("test message with unavailable session")
		output := buf.String()
		require.Contains(t, output, "test message with unavailable session",
			"expected log message to be captured by the app logger")
	})
}
