package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
)

type contextKey string

const requestIDContextKey contextKey = "request_id"

func parseLogLevel(raw string) (slog.Level, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "info":
		return slog.LevelInfo, true
	case "debug":
		return slog.LevelDebug, true
	case "warn", "warning":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	default:
		return slog.LevelInfo, false
	}
}

func setupLogger() {
	level, ok := parseLogLevel(logLevel)
	if !ok {
		fmt.Fprintf(os.Stderr, "invalid LOG_LEVEL %q; defaulting to info\n", logLevel)
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}

func withRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey, requestID)
}

func requestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value, ok := ctx.Value(requestIDContextKey).(string); ok {
		return value
	}
	return ""
}

func generateRequestID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return hex.EncodeToString(buf)
}

func redactRemoteAddr(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	digest := sha256.Sum256([]byte(host))
	return hex.EncodeToString(digest[:8])
}
