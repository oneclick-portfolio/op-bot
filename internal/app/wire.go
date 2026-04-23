package app

import (
	"net/http"
	"op-bot/internal/httpapi"
)

type Dependencies struct {
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

func NewHTTPHandler(deps Dependencies, middleware func(http.Handler) http.Handler) http.Handler {
	router := httpapi.NewRouter(httpapi.Handlers{
		AuthGitHubStart:    deps.AuthGitHubStart,
		AuthGitHubCallback: deps.AuthGitHubCallback,
		APIGitHubMe:        deps.APIGitHubMe,
		APIGitHubRepos:     deps.APIGitHubRepos,
		APIGitHubLogout:    deps.APIGitHubLogout,
		APIResumeValidate:  deps.APIResumeValidate,
		APIResumeParsePDF:  deps.APIResumeParsePDF,
		APIGitHubDeploy:    deps.APIGitHubDeploy,
		SwaggerUI:          deps.SwaggerUI,
		OpenAPISpec:        deps.OpenAPISpec,
	})
	return middleware(router)
}
