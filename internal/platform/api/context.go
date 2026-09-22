package api

import "context"

type ctxKey int

const (
	requestIDKey ctxKey = iota
	clientIPKey
	bodyLimitKey
)

// RequestIDFromContext returns the request ID assigned by the RequestID
// middleware, or "" outside a request.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// ClientIP returns the client IP resolved by the RealIP middleware, or "".
func ClientIP(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey).(string)
	return ip
}

// defaultBodyLimit returns the router-wide request body limit.
func defaultBodyLimit(ctx context.Context) int64 {
	if n, ok := ctx.Value(bodyLimitKey).(int64); ok && n > 0 {
		return n
	}
	return fallbackBodyLimit
}

// fallbackBodyLimit applies when a handler runs outside a router built by
// NewRouter (for example in a unit test).
const fallbackBodyLimit int64 = 1 << 20
