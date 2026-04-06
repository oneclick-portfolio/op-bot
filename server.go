package main

import (
	"net/http"
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

	return mux
}
