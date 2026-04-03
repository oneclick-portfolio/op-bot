package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type ghError struct {
	Message string
	Status  int
}

func (e *ghError) Error() string {
	return e.Message
}

func ghRequest(token, endpoint, method string, body any) (map[string]any, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, "https://api.github.com"+endpoint, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var payload map[string]any
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &payload); err != nil {
			payload = map[string]any{"message": string(respBody)}
		}
	}

	if resp.StatusCode >= 400 {
		msg := fmt.Sprintf("GitHub API request failed with status %d", resp.StatusCode)
		if payload != nil {
			if m, ok := payload["message"].(string); ok && m != "" {
				msg = m
			}
		}
		return nil, &ghError{Message: msg, Status: resp.StatusCode}
	}

	return payload, nil
}

func getGitHubUserFromToken(token string) (map[string]any, error) {
	return ghRequest(token, "/user", http.MethodGet, nil)
}

func getGitHubInstallationsFromToken(token string) ([]map[string]any, error) {
	payload, err := ghRequest(token, "/user/installations", http.MethodGet, nil)
	if err != nil {
		return nil, err
	}
	installations, ok := payload["installations"].([]any)
	if !ok {
		return nil, nil
	}
	result := make([]map[string]any, 0, len(installations))
	for _, inst := range installations {
		if m, ok := inst.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result, nil
}

func pickInstallationForUser(installations []map[string]any, userLogin string) map[string]any {
	lowerLogin := strings.ToLower(userLogin)
	for _, inst := range installations {
		if account, ok := inst["account"].(map[string]any); ok {
			if login, ok := account["login"].(string); ok && strings.ToLower(login) == lowerLogin {
				return inst
			}
		}
	}
	if len(installations) > 0 {
		return installations[0]
	}
	return nil
}

func uploadFileToRepo(token, owner, repo, branch, filePath string, content []byte) error {
	parts := strings.Split(filePath, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	encodedPath := strings.Join(parts, "/")

	var existingSha string
	readURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, repo, encodedPath, url.QueryEscape(branch))
	req, _ := http.NewRequest(http.MethodGet, readURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var existing map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&existing); err == nil {
				if sha, ok := existing["sha"].(string); ok {
					existingSha = sha
				}
			}
		}
	}

	payload := map[string]any{
		"message": fmt.Sprintf("chore: add %s", filePath),
		"branch":  branch,
		"content": base64.StdEncoding.EncodeToString(content),
	}
	if existingSha != "" {
		payload["sha"] = existingSha
	}

	_, err = ghRequest(token, fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, encodedPath), http.MethodPut, payload)
	return err
}

type deployParams struct {
	Theme          string `json:"theme"`
	RepositoryName string `json:"repositoryName"`
	PrivateRepo    bool   `json:"privateRepo"`
	ResumeData     any    `json:"resumeData"`
}

type deployResult struct {
	RepositoryURL  string `json:"repositoryUrl"`
	PagesURL       string `json:"pagesUrl"`
	RepoFullName   string `json:"repoFullName"`
	ReusedExisting bool   `json:"reusedExistingRepo"`
	InstallationID any    `json:"installationId,omitempty"`
}

func createRepositoryAndDeployTheme(token, userLogin string, params deployParams) (*deployResult, error) {
	theme := params.Theme
	repositoryName := normalizeRepoName(params.RepositoryName)

	if !validateTheme(theme) {
		return nil, &ghError{Message: "Invalid theme selection.", Status: http.StatusBadRequest}
	}
	if repositoryName == "" {
		return nil, &ghError{Message: "Repository name is required.", Status: http.StatusBadRequest}
	}

	repo, err := ghRequest(token, "/user/repos", http.MethodPost, map[string]any{
		"name":        repositoryName,
		"private":     params.PrivateRepo,
		"auto_init":   true,
		"description": fmt.Sprintf("Portfolio site generated from %s theme", theme),
	})
	reusedExisting := false
	if err != nil {
		ghErr, ok := err.(*ghError)
		shouldTryExisting := ok && (ghErr.Status == http.StatusForbidden || ghErr.Status == http.StatusConflict || ghErr.Status == http.StatusUnprocessableEntity)
		if !shouldTryExisting {
			return nil, err
		}
		repo, err = ghRequest(token, fmt.Sprintf("/repos/%s/%s", userLogin, repositoryName), http.MethodGet, nil)
		if err != nil {
			return nil, &ghError{
				Message: fmt.Sprintf("Repository creation failed: %s. If the repository already exists, ensure the GitHub App is installed on it. If it does not exist, grant the app repository Administration (write) and install it for all repositories or create the repo manually first.", ghErr.Message),
				Status:  ghErr.Status,
			}
		}
		reusedExisting = true
	}

	branch := "main"
	if b, ok := repo["default_branch"].(string); ok && b != "" {
		branch = b
	}

	if params.ResumeData != nil {
		result := validateResumeData(params.ResumeData)
		if !result.Valid {
			firstErrors := strings.Join(result.Errors[:min(3, len(result.Errors))], "; ")
			if firstErrors == "" {
				firstErrors = "schema validation failed"
			}
			return nil, &ghError{Message: fmt.Sprintf("Uploaded resume JSON is invalid: %s", firstErrors), Status: http.StatusBadRequest}
		}
	}

	files, err := buildThemeBundle(theme, params.ResumeData)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if err := uploadFileToRepo(token, userLogin, repositoryName, branch, file.Path, file.Content); err != nil {
			return nil, err
		}
	}

	_, _ = ghRequest(token, fmt.Sprintf("/repos/%s/%s/pages", userLogin, repositoryName), http.MethodPost, map[string]any{"build_type": "workflow"})

	htmlURL, _ := repo["html_url"].(string)
	fullName, _ := repo["full_name"].(string)

	return &deployResult{
		RepositoryURL:  htmlURL,
		PagesURL:       fmt.Sprintf("https://%s.github.io/%s/", userLogin, repositoryName),
		RepoFullName:   fullName,
		ReusedExisting: reusedExisting,
	}, nil
}
