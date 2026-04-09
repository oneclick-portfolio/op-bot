package main

import (
	"log/slog"
	"net/http"
	"time"
)

func main() {
	setupLogger()

	handler := buildHandler(newServerMux())
	server := &http.Server{
		Addr:              httpAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	slog.Info("server.starting", "addr", httpAddr, "port", port)
	slog.Info("server.swagger_ui", "url", "http://localhost:"+port+"/swagger")
	if err := server.ListenAndServe(); err != nil {
		slog.Error("server.listen_failed", "error", err)
		return
	}
}
