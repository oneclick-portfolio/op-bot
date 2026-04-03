package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
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

	fmt.Printf("Server running at http://localhost:%s\n", port)
	fmt.Printf("Swagger UI available at http://localhost:%s/swagger\n", port)
	if err := server.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
