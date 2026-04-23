package logging

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"os"

	"op-bot/internal/appctx"
	"op-bot/internal/utils"
)

type contextKey string

const requestIDContextKey contextKey = "request_id"

// SetupLogger initializes the default logger with configured level and returns it.
func SetupLogger(logLevel string) *slog.Logger {
	level, ok := utils.ParseLogLevel(logLevel)
	if !ok {
		fmt.Fprintf(os.Stderr, "invalid LOG_LEVEL %q; defaulting to info\n", logLevel)
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

// WithRequestID adds a request ID to the context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey, requestID)
}

// RequestIDFromContext retrieves the request ID from the context.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value, ok := ctx.Value(requestIDContextKey).(string); ok {
		return value
	}
	return ""
}

// GenerateRequestID generates a random request ID.
func GenerateRequestID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return hex.EncodeToString(buf)
}

// RedactRemoteAddr redacts the remote address for logging by hashing the host portion.
func RedactRemoteAddr(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	digest := sha256.Sum256([]byte(host))
	return hex.EncodeToString(digest[:8])
}

// LoggingMiddleware is a middleware that adds request ID logging.
// This will be moved to internal/httpapi/middleware as part of Phase 3.
type LoggingMiddleware struct {
	ctx *appctx.AppContext
}

// NewLoggingMiddleware creates a new logging middleware.
func NewLoggingMiddleware(ctx *appctx.AppContext) *LoggingMiddleware {
	return &LoggingMiddleware{ctx: ctx}
}
