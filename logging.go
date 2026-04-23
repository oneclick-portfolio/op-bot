package main

import (
	"context"
	"op-bot/internal/logging"
)

// setupLogger is a wrapper around the internal logging setup for backwards compatibility.
// This will be removed in Phase 5 when main.go is fully updated.
func setupLogger() {
	logging.SetupLogger("") // Uses default or env LOG_LEVEL
}

// withRequestID is a wrapper for backwards compatibility.
func withRequestID(ctx context.Context, requestID string) context.Context {
	return logging.WithRequestID(ctx, requestID)
}

// requestIDFromContext is a wrapper for backwards compatibility.
func requestIDFromContext(ctx context.Context) string {
	return logging.RequestIDFromContext(ctx)
}

// generateRequestID is a wrapper for backwards compatibility.
func generateRequestID() string {
	return logging.GenerateRequestID()
}

// redactRemoteAddr is a wrapper for backwards compatibility.
func redactRemoteAddr(addr string) string {
	return logging.RedactRemoteAddr(addr)
}
