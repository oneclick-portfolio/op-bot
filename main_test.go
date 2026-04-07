package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func parseErrorResponse(t *testing.T, rr *httptest.ResponseRecorder) apiErrorResponse {
	t.Helper()
	var payload apiErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	return payload
}

func TestValidateResumeData(t *testing.T) {
	t.Skip("Skipping - requires external schema fetch from rxresu.me")

	var resumeData any
	// Use a minimal valid resume structure for testing
	resumeJSON := `{
		"basics": {"name": "Test User"},
		"work": [],
		"education": [],
		"skills": [],
		"projects": []
	}`
	if err := json.Unmarshal([]byte(resumeJSON), &resumeData); err != nil {
		t.Fatalf("failed to parse resume JSON: %v", err)
	}

	result := validateResumeData(resumeData)
	if !result.Valid {
		t.Fatalf("schema validation failed: %v", result.Errors)
	}
}

func TestNormalizeRepoName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"My Portfolio", "my-portfolio"},
		{"test--name", "test-name"},
		{"-test-", "test"},
		{"Test Name With Spaces", "test-name-with-spaces"},
		{"special@chars#here", "special-chars-here"},
	}

	for _, tt := range tests {
		result := normalizeRepoName(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeRepoName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestValidateTheme(t *testing.T) {
	if !validateTheme("modern") {
		t.Error("Expected 'modern' theme to be valid")
	}
	if !validateTheme("graphic") {
		t.Error("Expected 'graphic' theme to be valid")
	}
	if validateTheme("invalid") {
		t.Error("Expected 'invalid' theme to be invalid")
	}
}

func TestGetThemeLabel(t *testing.T) {
	if got := getThemeLabel("modern"); got != "Modern" {
		t.Errorf("getThemeLabel(modern) = %q, want %q", got, "Modern")
	}
	if got := getThemeLabel(""); got != "" {
		t.Errorf("getThemeLabel('') = %q, want ''", got)
	}
}

func TestValidateResumeDataInvalid(t *testing.T) {
	result := validateResumeData(nil)
	if result.Valid {
		t.Error("Expected nil to be invalid")
	}

	result = validateResumeData([]any{1, 2, 3})
	if result.Valid {
		t.Error("Expected array to be invalid")
	}

	result = validateResumeData("string")
	if result.Valid {
		t.Error("Expected string to be invalid")
	}
}

func TestDefaultErrorCode(t *testing.T) {
	tests := []struct {
		status int
		want   string
	}{
		{http.StatusBadRequest, "BAD_REQUEST"},
		{http.StatusUnauthorized, "UNAUTHORIZED"},
		{http.StatusNotFound, "NOT_FOUND"},
		{http.StatusInternalServerError, "INTERNAL_ERROR"},
		{http.StatusTeapot, "REQUEST_FAILED"},
	}

	for _, tt := range tests {
		if got := defaultErrorCode(tt.status); got != tt.want {
			t.Errorf("defaultErrorCode(%d) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestFormatSchemaErrors(t *testing.T) {
	if got := formatSchemaErrors(nil); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}

	err := errors.New(" line1 \n\n line2 ")
	got := formatSchemaErrors(err)
	if len(got) != 2 || got[0] != "line1" || got[1] != "line2" {
		t.Fatalf("unexpected formatted errors: %#v", got)
	}
}

func TestWriteErrorResponseShape(t *testing.T) {
	rr := httptest.NewRecorder()
	writeError(rr, http.StatusBadRequest, "bad input", "field is required")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	payload := parseErrorResponse(t, rr)
	if payload.Error.Code != "BAD_REQUEST" {
		t.Fatalf("code = %q, want BAD_REQUEST", payload.Error.Code)
	}
	if payload.Error.Message != "bad input" {
		t.Fatalf("message = %q, want bad input", payload.Error.Message)
	}
	if len(payload.Error.Details) != 1 {
		t.Fatalf("details len = %d, want 1", len(payload.Error.Details))
	}
}

func TestCookieHelpers(t *testing.T) {
	rr := httptest.NewRecorder()
	setCookie(rr, "k", "v", 60, false)
	resp := rr.Result()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range resp.Cookies() {
		req.AddCookie(c)
	}

	if got := getCookie(req, "k"); got != "v" {
		t.Fatalf("getCookie = %q, want v", got)
	}

	rr2 := httptest.NewRecorder()
	clearCookie(rr2, "k")
	if len(rr2.Result().Cookies()) == 0 {
		t.Fatal("expected clear cookie header")
	}
}

func TestRequireGitHubAppConfig(t *testing.T) {
	oldID, oldSecret := appClientID, appClientSecret
	t.Cleanup(func() {
		appClientID, appClientSecret = oldID, oldSecret
	})

	appClientID, appClientSecret = "", ""
	rr := httptest.NewRecorder()
	if ok := requireGitHubAppConfig(rr); ok {
		t.Fatal("expected false when config is missing")
	}
	payload := parseErrorResponse(t, rr)
	if payload.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("error code = %q, want INTERNAL_ERROR", payload.Error.Code)
	}

	appClientID, appClientSecret = "id", "secret"
	rr = httptest.NewRecorder()
	if ok := requireGitHubAppConfig(rr); !ok {
		t.Fatal("expected true when config exists")
	}
}

func TestHandleAPIResumeValidateInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/resume/validate", bytes.NewBufferString("{invalid"))
	rr := httptest.NewRecorder()

	handleAPIResumeValidate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	payload := parseErrorResponse(t, rr)
	if payload.Error.Code != "BAD_REQUEST" {
		t.Fatalf("code = %q, want BAD_REQUEST", payload.Error.Code)
	}
	if !strings.Contains(strings.ToLower(payload.Error.Message), "invalid json") {
		t.Fatalf("message = %q, expected invalid json", payload.Error.Message)
	}
}

func TestNewServerMuxSwaggerSpecRoute(t *testing.T) {
	mux := newServerMux()
	req := httptest.NewRequest(http.MethodGet, "/swagger/openapi.json", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type = %q, want application/json", ct)
	}
}

func TestNewServerMuxGitHubReposRoute(t *testing.T) {
	mux := newServerMux()
	req := httptest.NewRequest(http.MethodGet, "/api/github/repos", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	payload := parseErrorResponse(t, rr)
	if payload.Error.Code != "UNAUTHORIZED" {
		t.Fatalf("error code = %q, want UNAUTHORIZED", payload.Error.Code)
	}
}

func TestNormalizeInstallURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"https://github.com/apps/myapp", "https://github.com/apps/myapp/installations/new"},
		{"https://github.com/apps/myapp/", "https://github.com/apps/myapp/installations/new"},
		{"https://github.com/apps/myapp/installations/new", "https://github.com/apps/myapp/installations/new"},
		{"https://example.com/custom", "https://example.com/custom"},
	}

	for _, tt := range tests {
		result := normalizeInstallURL(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeInstallURL(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestValidateTestFile(t *testing.T) {
	data, err := os.ReadFile("test/test.json")
	if err != nil {
		t.Fatalf("failed to read test.json: %v", err)
	}

	var resumeData any
	if err := json.Unmarshal(data, &resumeData); err != nil {
		t.Fatalf("failed to parse resume JSON: %v", err)
	}

	result := validateResumeData(resumeData)
	if !result.Valid {
		t.Fatalf("schema validation failed: \n- %s", strings.Join(result.Errors, "\n- "))
	}
}
