package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
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

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   slog.Level
		wantOK bool
	}{
		{name: "empty defaults to info", input: "", want: slog.LevelInfo, wantOK: true},
		{name: "info", input: "info", want: slog.LevelInfo, wantOK: true},
		{name: "debug", input: "debug", want: slog.LevelDebug, wantOK: true},
		{name: "warn", input: "warn", want: slog.LevelWarn, wantOK: true},
		{name: "warning", input: "warning", want: slog.LevelWarn, wantOK: true},
		{name: "error", input: "error", want: slog.LevelError, wantOK: true},
		{name: "mixed case trimmed", input: " WARN ", want: slog.LevelWarn, wantOK: true},
		{name: "invalid falls back to info", input: "verbose", want: slog.LevelInfo, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseLogLevel(tt.input)
			if got != tt.want {
				t.Fatalf("parseLogLevel(%q) level = %v, want %v", tt.input, got, tt.want)
			}
			if ok != tt.wantOK {
				t.Fatalf("parseLogLevel(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
		})
	}
}

func TestValidateThemeFiles(t *testing.T) {
	tests := []struct {
		name    string
		entries []fileEntry
		wantErr bool
	}{
		{
			name: "valid with style.css",
			entries: []fileEntry{
				{Path: "index.html"},
				{Path: "app.js"},
				{Path: "style.css"},
			},
			wantErr: false,
		},
		{
			name: "valid with styles.css",
			entries: []fileEntry{
				{Path: "index.html"},
				{Path: "nested/app.js"},
				{Path: "assets/styles.css"},
			},
			wantErr: false,
		},
		{
			name: "missing index",
			entries: []fileEntry{
				{Path: "app.js"},
				{Path: "style.css"},
			},
			wantErr: true,
		},
		{
			name: "missing app",
			entries: []fileEntry{
				{Path: "index.html"},
				{Path: "style.css"},
			},
			wantErr: true,
		},
		{
			name: "missing style",
			entries: []fileEntry{
				{Path: "index.html"},
				{Path: "app.js"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateThemeFiles(tt.entries)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateThemeFiles() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseThemeRepoLink(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantRepo   string
		wantRef    string
		wantSubDir string
		wantError  bool
	}{
		{
			name:     "repo root URL uses default main ref",
			input:    "https://github.com/oneclick-portfolio/awesome-github-portfolio",
			wantRepo: "oneclick-portfolio/awesome-github-portfolio",
			wantRef:  "main",
		},
		{
			name:     "tree URL with single-segment ref",
			input:    "https://github.com/oneclick-portfolio/awesome-github-portfolio/tree/main",
			wantRepo: "oneclick-portfolio/awesome-github-portfolio",
			wantRef:  "main",
		},
		{
			name:       "tree URL with ref and theme subfolder",
			input:      "https://github.com/oneclick-portfolio/awesome-github-portfolio/tree/main/themes/graphic",
			wantRepo:   "oneclick-portfolio/awesome-github-portfolio",
			wantRef:    "main",
			wantSubDir: "themes/graphic",
		},
		{
			name:       "tree URL with ref and nested subfolder",
			input:      "https://github.com/oneclick-portfolio/awesome-github-portfolio/tree/develop/themes/vscode",
			wantRepo:   "oneclick-portfolio/awesome-github-portfolio",
			wantRef:    "develop",
			wantSubDir: "themes/vscode",
		},
		{
			name:       "tree URL with multi-segment ref rejected",
			input:      "https://github.com/org/repo/tree/release/v1",
			wantRepo:   "org/repo",
			wantRef:    "release",
			wantSubDir: "v1",
		},
		{
			name:      "missing URL",
			input:     "",
			wantError: true,
		},
		{
			name:      "non-github URL rejected",
			input:     "https://gitlab.com/org/repo",
			wantError: true,
		},
		{
			name:      "query string rejected",
			input:     "https://github.com/org/repo?tab=readme",
			wantError: true,
		},
		{
			name:      "unsupported extra path rejected",
			input:     "https://github.com/org/repo/issues",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseThemeRepoLink(tt.input)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil and value %+v", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got.Repo != tt.wantRepo {
				t.Fatalf("repo = %q, want %q", got.Repo, tt.wantRepo)
			}
			if got.Ref != tt.wantRef {
				t.Fatalf("ref = %q, want %q", got.Ref, tt.wantRef)
			}
			if got.SubDir != tt.wantSubDir {
				t.Fatalf("subDir = %q, want %q", got.SubDir, tt.wantSubDir)
			}
		})
	}
}

func TestPickInstallationForUserPrefersMatchingActiveUserInstallation(t *testing.T) {
	installations := []map[string]any{
		{
			"id":              float64(21),
			"access_ended_at": nil,
			"account": map[string]any{
				"login": "some-org",
				"type":  "Organization",
			},
		},
		{
			"id":              float64(22),
			"access_ended_at": nil,
			"account": map[string]any{
				"login": "alice",
				"type":  "User",
			},
		},
	}

	chosen := pickInstallationForUser(installations, "Alice")
	if chosen == nil {
		t.Fatal("expected matching installation")
	}
	id, _ := chosen["id"].(float64)
	if int64(id) != 22 {
		t.Fatalf("expected installation id 22, got %v", chosen["id"])
	}
}

func TestPickInstallationForUserNoFallbackToNonMatchingInstallation(t *testing.T) {
	installations := []map[string]any{
		{
			"id": float64(31),
			"account": map[string]any{
				"login": "some-org",
				"type":  "Organization",
			},
		},
	}

	chosen := pickInstallationForUser(installations, "alice")
	if chosen != nil {
		t.Fatalf("expected nil installation, got %+v", chosen)
	}
}

func TestPickInstallationForUserSkipsRevokedInstallations(t *testing.T) {
	installations := []map[string]any{
		{
			"id":              float64(41),
			"access_ended_at": "2026-01-01T00:00:00Z",
			"account": map[string]any{
				"login": "alice",
				"type":  "User",
			},
		},
	}

	chosen := pickInstallationForUser(installations, "alice")
	if chosen != nil {
		t.Fatalf("expected nil for revoked installation, got %+v", chosen)
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

// --- PDF Parse handler tests ---

// createMultipartPDF builds a multipart/form-data request body with the given
// content as a file upload named "file".
func createMultipartPDF(t *testing.T, filename string, content []byte, contentType string) (*bytes.Buffer, string) {
	t.Helper()
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	h := make(map[string][]string)
	h["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename)}
	if contentType != "" {
		h["Content-Type"] = []string{contentType}
	}
	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatalf("failed to create form part: %v", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(content)); err != nil {
		t.Fatalf("failed to write form part: %v", err)
	}
	writer.Close()
	return body, writer.FormDataContentType()
}

func TestHandleAPIResumeParsePDF_NoFile(t *testing.T) {
	// Send a multipart request with no file field at all.
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/resume/parse", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()

	oldKey := googleAPIKey
	googleAPIKey = "test-key"
	t.Cleanup(func() { googleAPIKey = oldKey })

	handleAPIResumeParsePDF(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	payload := parseErrorResponse(t, rr)
	if !strings.Contains(strings.ToLower(payload.Error.Message), "file") {
		t.Fatalf("message = %q, expected mention of file", payload.Error.Message)
	}
}

func TestHandleAPIResumeParsePDF_WrongContentType(t *testing.T) {
	body, ct := createMultipartPDF(t, "resume.txt", []byte("not a pdf"), "text/plain")

	req := httptest.NewRequest(http.MethodPost, "/api/resume/parse", body)
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()

	oldKey := googleAPIKey
	googleAPIKey = "test-key"
	t.Cleanup(func() { googleAPIKey = oldKey })

	handleAPIResumeParsePDF(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	payload := parseErrorResponse(t, rr)
	if !strings.Contains(strings.ToLower(payload.Error.Message), "pdf") {
		t.Fatalf("message = %q, expected mention of PDF", payload.Error.Message)
	}
}

func TestHandleAPIResumeParsePDF_NotValidPDF(t *testing.T) {
	// Send application/pdf content-type but content that doesn't start with %PDF.
	body, ct := createMultipartPDF(t, "resume.pdf", []byte("this is not a real pdf"), "application/pdf")

	req := httptest.NewRequest(http.MethodPost, "/api/resume/parse", body)
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()

	oldKey := googleAPIKey
	googleAPIKey = "test-key"
	t.Cleanup(func() { googleAPIKey = oldKey })

	handleAPIResumeParsePDF(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	payload := parseErrorResponse(t, rr)
	if !strings.Contains(strings.ToLower(payload.Error.Message), "valid pdf") {
		t.Fatalf("message = %q, expected 'valid PDF' mention", payload.Error.Message)
	}
}

func TestHandleAPIResumeParsePDF_MissingAPIKey(t *testing.T) {
	// Create a valid-looking PDF (starts with %PDF).
	fakePDF := []byte("%PDF-1.4 fake content")
	body, ct := createMultipartPDF(t, "resume.pdf", fakePDF, "application/pdf")

	req := httptest.NewRequest(http.MethodPost, "/api/resume/parse", body)
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()

	oldKey := googleAPIKey
	googleAPIKey = ""
	t.Cleanup(func() { googleAPIKey = oldKey })

	handleAPIResumeParsePDF(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
	payload := parseErrorResponse(t, rr)
	if !strings.Contains(strings.ToLower(payload.Error.Message), "google_api_key") {
		t.Fatalf("message = %q, expected mention of GOOGLE_API_KEY", payload.Error.Message)
	}
}

func TestHandleAPIResumeParsePDF_FileTooLarge(t *testing.T) {
	// Create content >5 MB.
	largeContent := make([]byte, 6<<20)
	copy(largeContent[:4], []byte("%PDF"))
	body, ct := createMultipartPDF(t, "resume.pdf", largeContent, "application/pdf")

	req := httptest.NewRequest(http.MethodPost, "/api/resume/parse", body)
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()

	oldKey := googleAPIKey
	googleAPIKey = "test-key"
	t.Cleanup(func() { googleAPIKey = oldKey })

	handleAPIResumeParsePDF(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	payload := parseErrorResponse(t, rr)
	if !strings.Contains(strings.ToLower(payload.Error.Message), "large") && !strings.Contains(strings.ToLower(payload.Error.Message), "5mb") {
		t.Fatalf("message = %q, expected size limit mention", payload.Error.Message)
	}
}

func TestHandleAPIResumeParsePDF_RouteExists(t *testing.T) {
	mux := newServerMux()

	// Verify the route is registered by making a POST request.
	// Without API key, we should get 500 (not 404/405).
	oldKey := googleAPIKey
	googleAPIKey = ""
	t.Cleanup(func() { googleAPIKey = oldKey })

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/resume/parse", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Should not be 404/405 — route must be registered.
	if rr.Code == http.StatusNotFound || rr.Code == http.StatusMethodNotAllowed {
		t.Fatalf("route /api/resume/parse not registered, got status %d", rr.Code)
	}
}

// --- ExtractJSON tests ---

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain JSON",
			input: `{"name": "test"}`,
			want:  `{"name": "test"}`,
		},
		{
			name:  "markdown-wrapped JSON",
			input: "```json\n{\"name\": \"test\"}\n```",
			want:  `{"name": "test"}`,
		},
		{
			name:  "text before and after JSON",
			input: "Here is the result:\n{\"basics\": {\"name\": \"Alice\"}}\nDone.",
			want:  `{"basics": {"name": "Alice"}}`,
		},
		{
			name:  "nested braces",
			input: `{"a": {"b": {"c": 1}}}`,
			want:  `{"a": {"b": {"c": 1}}}`,
		},
		{
			name:  "no JSON at all",
			input: "just plain text",
			want:  "just plain text",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractJSON(tt.input)
			if got != tt.want {
				t.Errorf("ExtractJSON(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- ParsePDFToJSON unit tests ---

func TestParsePDFToJSON_NoAPIKey(t *testing.T) {
	_, err := ParsePDFToJSON(context.Background(), "", []byte("%PDF-1.4 content"))
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
	if !strings.Contains(err.Error(), "API key") {
		t.Fatalf("error = %q, expected API key mention", err)
	}
}
