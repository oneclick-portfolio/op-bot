package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
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

func getInstallationRepositories(token string) ([]map[string]any, error) {
	payload, err := ghRequest(token, "/installation/repositories?per_page=100", http.MethodGet, nil)
	if err != nil {
		return nil, err
	}
	repositories, ok := payload["repositories"].([]any)
	if !ok {
		return nil, nil
	}
	result := make([]map[string]any, 0, len(repositories))
	for _, repo := range repositories {
		if m, ok := repo.(map[string]any); ok {
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

func commitAllFiles(token, owner, repo, branch, message string, files []fileEntry) error {
	var refResp map[string]any
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		refResp, err = ghRequest(token, fmt.Sprintf("/repos/%s/%s/git/ref/heads/%s", owner, repo, url.PathEscape(branch)), http.MethodGet, nil)
		if err == nil {
			break
		}
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	if err != nil {
		return fmt.Errorf("unable to get branch ref: %w", err)
	}
	refObj, _ := refResp["object"].(map[string]any)
	parentSha, _ := refObj["sha"].(string)
	if parentSha == "" {
		return fmt.Errorf("unable to resolve HEAD sha for branch %s", branch)
	}

	commitResp, err := ghRequest(token, fmt.Sprintf("/repos/%s/%s/git/commits/%s", owner, repo, parentSha), http.MethodGet, nil)
	if err != nil {
		return fmt.Errorf("unable to get parent commit: %w", err)
	}
	treeObj, _ := commitResp["tree"].(map[string]any)
	baseTreeSha, _ := treeObj["sha"].(string)

	treeItems := make([]map[string]any, 0, len(files))
	for _, f := range files {
		blobResp, err := ghRequest(token, fmt.Sprintf("/repos/%s/%s/git/blobs", owner, repo), http.MethodPost, map[string]any{
			"content":  base64.StdEncoding.EncodeToString(f.Content),
			"encoding": "base64",
		})
		if err != nil {
			return fmt.Errorf("unable to create blob for %s: %w", f.Path, err)
		}
		blobSha, _ := blobResp["sha"].(string)

		treeItems = append(treeItems, map[string]any{
			"path": f.Path,
			"mode": "100644",
			"type": "blob",
			"sha":  blobSha,
		})
	}

	treeResp, err := ghRequest(token, fmt.Sprintf("/repos/%s/%s/git/trees", owner, repo), http.MethodPost, map[string]any{
		"base_tree": baseTreeSha,
		"tree":      treeItems,
	})
	if err != nil {
		return fmt.Errorf("unable to create tree: %w", err)
	}
	newTreeSha, _ := treeResp["sha"].(string)

	newCommitResp, err := ghRequest(token, fmt.Sprintf("/repos/%s/%s/git/commits", owner, repo), http.MethodPost, map[string]any{
		"message": message,
		"tree":    newTreeSha,
		"parents": []string{parentSha},
	})
	if err != nil {
		return fmt.Errorf("unable to create commit: %w", err)
	}
	newCommitSha, _ := newCommitResp["sha"].(string)

	_, err = ghRequest(token, fmt.Sprintf("/repos/%s/%s/git/refs/heads/%s", owner, repo, url.PathEscape(branch)), http.MethodPatch, map[string]any{
		"sha":   newCommitSha,
		"force": false,
	})
	if err != nil {
		return fmt.Errorf("unable to update branch ref: %w", err)
	}

	return nil
}

type deployParams struct {
	Theme           string `json:"theme"`
	RepositoryName  string `json:"repositoryName"`
	RepositoryOwner string `json:"repositoryOwner,omitempty"`
	PrivateRepo     bool   `json:"privateRepo"`
	ResumeData      any    `json:"resumeData"`
}

type deployResult struct {
	RepositoryURL  string `json:"repositoryUrl"`
	PagesURL       string `json:"pagesUrl"`
	RepoFullName   string `json:"repoFullName"`
	ReusedExisting bool   `json:"reusedExistingRepo"`
	InstallationID any    `json:"installationId,omitempty"`
}

func createRepositoryAndDeployTheme(userLogin string, installationID int64, params deployParams) (*deployResult, error) {
	theme := params.Theme
	repositoryName := normalizeRepoName(params.RepositoryName)
	repositoryOwner := strings.TrimSpace(params.RepositoryOwner)
	if repositoryOwner == "" {
		repositoryOwner = userLogin
	}

	if !validateTheme(theme) {
		return nil, &ghError{Message: "Invalid theme selection.", Status: http.StatusBadRequest}
	}
	if repositoryName == "" {
		return nil, &ghError{Message: "Repository name is required.", Status: http.StatusBadRequest}
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

	bundle, err := buildThemeBundle(theme, repositoryOwner, repositoryName, params.ResumeData)
	if err != nil {
		return nil, &ghError{Message: fmt.Sprintf("Unable to build %s theme bundle: %v", theme, err), Status: http.StatusInternalServerError}
	}
	log.Printf("deploy.theme bundle_ready theme=%s source=%s@%s files=%d", theme, themeSourceRepo, themeSourceRef, len(bundle))

	instToken, err := getInstallationToken(installationID)
	if err != nil {
		return nil, &ghError{Message: fmt.Sprintf("Unable to obtain bot installation token: %v", err), Status: http.StatusInternalServerError}
	}
	log.Printf("deploy.bot_token_ready installation_id=%d", installationID)
	uploadToken := instToken

	reusedExisting := false
	repo, err := ghRequest(uploadToken, fmt.Sprintf("/repos/%s/%s", repositoryOwner, repositoryName), http.MethodGet, nil)
	if err != nil {
		log.Printf("deploy.repo_not_found repo=%s/%s, creating...", repositoryOwner, repositoryName)
		repo, err = ghRequest(uploadToken, "/user/repos", http.MethodPost, map[string]any{
			"name":         repositoryName,
			"description":  fmt.Sprintf("Portfolio site deployed with the %s theme", theme),
			"private":      params.PrivateRepo,
			"auto_init":    true,
			"has_issues":   false,
			"has_projects": false,
			"has_wiki":     false,
		})
		if err != nil {
			return nil, &ghError{Message: fmt.Sprintf("Unable to create repository: %v", err), Status: http.StatusBadGateway}
		}
		log.Printf("deploy.repo_created repo=%s/%s", repositoryOwner, repositoryName)
	} else {
		reusedExisting = true
	}

	commitMsg := fmt.Sprintf("chore: deploy %s theme portfolio", theme)
	if err := commitAllFiles(uploadToken, repositoryOwner, repositoryName, "main", commitMsg, bundle); err != nil {
		if ghErr, ok := err.(*ghError); ok {
			return nil, ghErr
		}
		return nil, &ghError{Message: fmt.Sprintf("Unable to commit theme files: %v", err), Status: http.StatusBadGateway}
	}
	log.Printf("deploy.theme committed theme=%s repo=%s/%s files=%d", theme, repositoryOwner, repositoryName, len(bundle))

	_, _ = ghRequest(uploadToken, fmt.Sprintf("/repos/%s/%s/pages", repositoryOwner, repositoryName), http.MethodPost, map[string]any{"build_type": "workflow"})

	htmlURL, _ := repo["html_url"].(string)
	fullName, _ := repo["full_name"].(string)

	return &deployResult{
		RepositoryURL:  htmlURL,
		PagesURL:       fmt.Sprintf("https://%s.github.io/%s/", repositoryOwner, repositoryName),
		RepoFullName:   fullName,
		ReusedExisting: reusedExisting,
	}, nil
}
