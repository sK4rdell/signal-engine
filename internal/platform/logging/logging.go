// Package logging builds the structured slog logger and carries a
// request-scoped logger through context.Context.
//
// Middleware creates a scope per request; later layers (authentication,
// account resolution, feature code) add attributes to that scope, and every
// log line written through FromContext carries all of them. The request log
// line written at the end of the request sees the attributes added along the
// way, which is why the scope is a shared mutable value rather than a chain
// of derived loggers.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
)

// New returns a logger writing to w in the requested format.
//
// Level and format are validated by config; unrecognised values fall back
// to info and JSON rather than failing at this layer.
func New(w io.Writer, level, format string) *slog.Logger {
	if w == nil {
		w = os.Stderr
	}
	opts := &slog.HandlerOptions{Level: parseLevel(level)}

	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}
	return slog.New(handler)
}

// Discard returns a logger that drops everything. Useful in tests.
func Discard() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type scopeKey struct{}

// scope is the mutable per-request attribute set.
type scope struct {
	base *slog.Logger

	mu    sync.Mutex
	attrs []any
}

// NewScope returns a context carrying a fresh logging scope rooted at base.
func NewScope(ctx context.Context, base *slog.Logger) context.Context {
	return context.WithValue(ctx, scopeKey{}, &scope{base: base})
}

// WithLogger returns a context whose scope is rooted at logger. It is a
// convenience for code that has no request scope, such as workers and tests.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return NewScope(ctx, logger)
}

// Add attaches key/value attributes to the current scope. Every logger
// obtained from FromContext afterwards, in this or any derived context,
// includes them. Without a scope Add is a no-op.
func Add(ctx context.Context, attrs ...any) {
	s, ok := ctx.Value(scopeKey{}).(*scope)
	if !ok {
		return
	}
	s.mu.Lock()
	s.attrs = append(s.attrs, attrs...)
	s.mu.Unlock()
}

// FromContext returns the scoped logger, or slog.Default when the context
// carries no scope.
func FromContext(ctx context.Context) *slog.Logger {
	s, ok := ctx.Value(scopeKey{}).(*scope)
	if !ok {
		return slog.Default()
	}
	s.mu.Lock()
	attrs := append([]any(nil), s.attrs...)
	s.mu.Unlock()
	if len(attrs) == 0 {
		return s.base
	}
	return s.base.With(attrs...)
}
