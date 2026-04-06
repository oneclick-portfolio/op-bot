package main

import (
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

func buildHandler(next http.Handler) http.Handler {
	handler := next
	handler = recoverMiddleware(handler)
	handler = securityHeadersMiddleware(handler)
	handler = corsMiddleware(handler)
	handler = requestLoggerMiddleware(handler)
	return handler
}

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

func requestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		rw := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(rw, r)

		status := rw.status
		if status == 0 {
			status = http.StatusOK
		}

		log.Printf("request method=%s path=%s status=%d duration=%s bytes=%d remote=%s", r.Method, r.URL.Path, status, time.Since(started).Round(time.Millisecond), rw.bytesWritten, r.RemoteAddr)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	allowedMethods := "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	allowedHeaders := "Accept, Authorization, Content-Type, X-Requested-With"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if allowedOrigin(origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
				w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
				w.Header().Set("Access-Control-Max-Age", "600")
				if corsCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}
		}

		if r.Method == http.MethodOptions {
			if origin != "" && allowedOrigin(origin) {
				w.WriteHeader(http.StatusNoContent)
			} else {
				w.WriteHeader(http.StatusForbidden)
			}
			return
		}

		next.ServeHTTP(w, r)
	})
}

func allowedOrigin(origin string) bool {
	for _, allowed := range corsOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-site")
		next.ServeHTTP(w, r)
	})
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic recovered method=%s path=%s err=%v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
				writeError(w, http.StatusInternalServerError, "Internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
