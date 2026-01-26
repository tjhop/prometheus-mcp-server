package mcp

import (
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/common/promslog"

	"github.com/tjhop/prometheus-mcp-server/pkg/log/multihandler"
)

// clientLoggingInterval is the minimum interval between log messages sent to
// the MCP client. This rate limits client notifications to prevent flooding.
const clientLoggingInterval = 100 * time.Millisecond

// getClientLogger creates an slog.Logger that sends logs to the MCP client via
// protocol notifications. Returns nil if the session cannot be obtained, for
// some reason.  The logger uses rate limiting to prevent flooding clients with
// messages.  The loggerName appears in the "logger" field of log
// notifications.
func getClientLogger(req *mcp.CallToolRequest, loggerName string) *slog.Logger {
	if req == nil {
		return nil
	}

	serverSession, ok := req.GetSession().(*mcp.ServerSession)
	if !ok || serverSession == nil {
		return nil
	}

	return slog.New(mcp.NewLoggingHandler(serverSession, &mcp.LoggingHandlerOptions{
		LoggerName:  loggerName,
		MinInterval: clientLoggingInterval,
	}))
}

// getChainedLogger creates an slog.Logger that logs to both the application
// logger and the MCP client. If client logging is not available for some
// reason (no session, etc.), returns the application logger unchanged. If
// logger is nil, it'll attempt MCP client notification only logging.
//
// This enables tool handlers to log messages that appear both in server logs
// and are sent to the connected LLM client as notifications.
func getChainedLogger(logger *slog.Logger, req *mcp.CallToolRequest, loggerName string) *slog.Logger {
	var chainedLogger *slog.Logger
	clientLogger := getClientLogger(req, loggerName)

	switch {
	case logger == nil && clientLogger == nil:
		chainedLogger = promslog.NewNopLogger()
	case logger != nil && clientLogger == nil:
		chainedLogger = logger
	case logger == nil && clientLogger != nil:
		chainedLogger = clientLogger
	default:
		chainedLogger = slog.New(multihandler.New(logger.Handler(), clientLogger.Handler()))
	}

	return chainedLogger
}
