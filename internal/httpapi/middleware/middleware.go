package middleware

import (
	"log/slog"
	"net/http"
	"op-bot/internal/httpapi/response"
	"op-bot/internal/logging"
	"runtime/debug"
	"time"
)

// responseRecorder wraps http.ResponseWriter to capture response metadata.
type responseRecorder struct {
	http.ResponseWriter
	status       int
	bytesWritten int
}

func (rw *responseRecorder) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseRecorder) Write(data []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(data)
	rw.bytesWritten += n
	return n, err
}

// RequestLoggerMiddleware logs HTTP requests with request ID and duration.
// It requires callback functions for generating request IDs and redacting addresses.
type RequestLogger struct {
	generateRequestID func() string
	redactAddr        func(string) string
}

// NewRequestLogger creates a new request logger middleware.
func NewRequestLogger(generateRequestID func() string, redactAddr func(string) string) *RequestLogger {
	return &RequestLogger{
		generateRequestID: generateRequestID,
		redactAddr:        redactAddr,
	}
}

// Handler returns the middleware handler.
func (rl *RequestLogger) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = rl.generateRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)
		r = r.WithContext(logging.WithRequestID(r.Context(), requestID))

		started := time.Now()
		rw := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(rw, r)

		status := rw.status
		if status == 0 {
			status = http.StatusOK
		}

		slog.InfoContext(r.Context(), "http.request",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", time.Since(started).Milliseconds(),
			"bytes", rw.bytesWritten,
			"remote_hash", rl.redactAddr(r.RemoteAddr),
		)
	})
}

// SecurityHeadersMiddleware adds security headers to responses.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-site")
		next.ServeHTTP(w, r)
	})
}

// RecoverMiddleware catches panics and returns an error response.
func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.ErrorContext(r.Context(), "http.panic_recovered",
					"request_id", logging.RequestIDFromContext(r.Context()),
					"method", r.Method,
					"path", r.URL.Path,
					"error", rec,
					"stack", string(debug.Stack()),
				)
				response.WriteError(w, http.StatusInternalServerError, "Internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// CORSMiddleware handles Cross-Origin Resource Sharing (CORS) requests.
type CORSConfig struct {
	AllowedOrigins []string
	Credentials    bool
}

// NewCORSMiddleware creates a CORS middleware with the given configuration.
func NewCORSMiddleware(config CORSConfig) func(http.Handler) http.Handler {
	allowedMethods := "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	allowedHeaders := "Accept, Authorization, Content-Type, X-Requested-With"

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if isAllowedOrigin(origin, config.AllowedOrigins) {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Add("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
					w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
					w.Header().Set("Access-Control-Max-Age", "600")
					if config.Credentials {
						w.Header().Set("Access-Control-Allow-Credentials", "true")
					}
				}
			}

			if r.Method == http.MethodOptions {
				if origin != "" && isAllowedOrigin(origin, config.AllowedOrigins) {
					w.WriteHeader(http.StatusNoContent)
				} else {
					w.WriteHeader(http.StatusForbidden)
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isAllowedOrigin(origin string, corsOrigins []string) bool {
	for _, allowed := range corsOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}
