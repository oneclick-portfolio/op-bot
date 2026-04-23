package main

import (
	"net/http"
	"op-bot/internal/httpapi/middleware"
)

// buildHandler composes the middleware chain for HTTP handlers.
// This is a wrapper that delegates to the internal middleware package.
func buildHandler(next http.Handler) http.Handler {
	handler := next
	handler = middleware.RecoverMiddleware(handler)
	handler = middleware.SecurityHeadersMiddleware(handler)

	// CORS middleware requires configuration from globals
	corsMiddleware := middleware.NewCORSMiddleware(middleware.CORSConfig{
		AllowedOrigins: corsOrigins,
		Credentials:    corsCredentials,
	})
	handler = corsMiddleware(handler)

	// Request logger middleware requires callback functions
	requestLogger := middleware.NewRequestLogger(generateRequestID, redactRemoteAddr)
	handler = requestLogger.Handler(handler)

	return handler
}
