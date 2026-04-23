package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestNewRouterRegistersRoutes(t *testing.T) {
	router := NewRouter(Handlers{
		AuthGitHubStart:    okHandler,
		AuthGitHubCallback: okHandler,
		APIGitHubMe:        okHandler,
		APIGitHubRepos:     okHandler,
		APIGitHubLogout:    okHandler,
		APIResumeValidate:  okHandler,
		APIResumeParsePDF:  okHandler,
		APIGitHubDeploy:    okHandler,
		SwaggerUI:          okHandler,
		OpenAPISpec:        okHandler,
	})

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/auth/github/start"},
		{http.MethodGet, "/auth/github/callback"},
		{http.MethodGet, "/api/github/me"},
		{http.MethodGet, "/api/github/repos"},
		{http.MethodPost, "/api/github/logout"},
		{http.MethodPost, "/api/resume/validate"},
		{http.MethodPost, "/api/resume/parse"},
		{http.MethodPost, "/api/github/deploy"},
		{http.MethodGet, "/swagger"},
		{http.MethodGet, "/swagger/"},
		{http.MethodGet, "/swagger/openapi.json"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s %s returned %d, want 200", tt.method, tt.path, rr.Code)
		}
	}
}
