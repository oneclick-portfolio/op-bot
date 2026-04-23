package httpapi

import "net/http"

type Handlers struct {
	AuthGitHubStart    http.HandlerFunc
	AuthGitHubCallback http.HandlerFunc
	APIGitHubMe        http.HandlerFunc
	APIGitHubRepos     http.HandlerFunc
	APIGitHubLogout    http.HandlerFunc
	APIResumeValidate  http.HandlerFunc
	APIResumeParsePDF  http.HandlerFunc
	APIGitHubDeploy    http.HandlerFunc
	SwaggerUI          http.HandlerFunc
	OpenAPISpec        http.HandlerFunc
}

func NewRouter(h Handlers) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /auth/github/start", h.AuthGitHubStart)
	mux.HandleFunc("GET /auth/github/callback", h.AuthGitHubCallback)
	mux.HandleFunc("GET /api/github/me", h.APIGitHubMe)
	mux.HandleFunc("GET /api/github/repos", h.APIGitHubRepos)
	mux.HandleFunc("POST /api/github/logout", h.APIGitHubLogout)
	mux.HandleFunc("POST /api/resume/validate", h.APIResumeValidate)
	mux.HandleFunc("POST /api/resume/parse", h.APIResumeParsePDF)
	mux.HandleFunc("POST /api/github/deploy", h.APIGitHubDeploy)

	mux.HandleFunc("GET /swagger", h.SwaggerUI)
	mux.HandleFunc("GET /swagger/", h.SwaggerUI)
	mux.HandleFunc("GET /swagger/openapi.json", h.OpenAPISpec)

	return mux
}
