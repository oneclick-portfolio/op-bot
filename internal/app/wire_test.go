package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func ok(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

func identity(h http.Handler) http.Handler { return h }

func TestNewHTTPHandlerBuildsRouterWithMiddleware(t *testing.T) {
	h := NewHTTPHandler(Dependencies{
		AuthGitHubStart:    ok,
		AuthGitHubCallback: ok,
		APIGitHubMe:        ok,
		APIGitHubRepos:     ok,
		APIGitHubLogout:    ok,
		APIResumeValidate:  ok,
		APIResumeParsePDF:  ok,
		APIGitHubDeploy:    ok,
		SwaggerUI:          ok,
		OpenAPISpec:        ok,
	}, identity)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/start", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rr.Code, http.StatusOK)
	}
}
