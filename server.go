package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func newServerMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /auth/github/start", handleAuthGitHubStart)
	mux.HandleFunc("GET /auth/github/callback", handleAuthGitHubCallback)
	mux.HandleFunc("GET /api/github/me", handleAPIGitHubMe)
	mux.HandleFunc("POST /api/github/logout", handleAPIGitHubLogout)
	mux.HandleFunc("POST /api/resume/validate", handleAPIResumeValidate)
	mux.HandleFunc("POST /api/github/deploy", handleAPIGitHubDeploy)

	mux.HandleFunc("GET /swagger", handleSwaggerUI)
	mux.HandleFunc("GET /swagger/", handleSwaggerUI)
	mux.HandleFunc("GET /swagger/openapi.json", handleOpenAPISpec)

	fs := http.FileServer(http.Dir(frontendDir))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fullPath := filepath.Join(frontendDir, r.URL.Path)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		if r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, "/api/") && !strings.HasPrefix(r.URL.Path, "/auth/") {
			pathWithHTML := fullPath + ".html"
			if _, err := os.Stat(pathWithHTML); err == nil {
				r.URL.Path = r.URL.Path + ".html"
				fs.ServeHTTP(w, r)
				return
			}
		}
		http.ServeFile(w, r, filepath.Join(frontendDir, "index.html"))
	}))

	return mux
}
