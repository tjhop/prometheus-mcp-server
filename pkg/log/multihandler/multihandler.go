// Package multihandler provides an slog.Handler that forwards log records
// to multiple underlying handlers simultaneously.
package multihandler

import (
	"context"
	"errors"
	"log/slog"
)

// TODO: This package is only a stop gap until go1.26 is released which
// includes support for multiple slog handlers in the stdlib.

// Handler is an slog.Handler that forwards records to multiple handlers.
// This enables logging to both local destinations (file/stderr) and to MCP
// clients simultaneously.
type Handler struct {
	handlers []slog.Handler
}

// New creates a handler that forwards to all provided handlers.
// Nil handlers are filtered out.
func New(handlers ...slog.Handler) *Handler {
	filtered := make([]slog.Handler, 0, len(handlers))
	for _, h := range handlers {
		if h != nil {
			filtered = append(filtered, h)
		}
	}
	return &Handler{handlers: filtered}
}

// Enabled returns true if any underlying handler is enabled for the level.
func (m *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	// The Handler is always enabled -- each backing handler is
	// checked to see if they're enabled at the given log level before
	// attempting to log with the handler.
	return true
}

// Handle forwards the record to all underlying handlers. It is up to the
// underlying handlers to properly report whether or not they are enabled for a
// given log level. Errors from all handlers are collected and joined for
// return. A non-nil error indicates at least one backing handler returned an
// error from it's respective Handle() call.
func (m *Handler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	rec := r.Clone()

	for _, h := range m.handlers {
		if h.Enabled(ctx, rec.Level) {
			errs = append(errs, h.Handle(ctx, rec))
		}
	}
	return errors.Join(errs...)
}

// WithAttrs returns a new Handler with attributes added to all handlers.
func (m *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &Handler{handlers: handlers}
}

// WithGroup returns a new Handler with a group added to all handlers.
func (m *Handler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &Handler{handlers: handlers}
}
