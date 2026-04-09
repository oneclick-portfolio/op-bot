package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type ghError struct {
	Message string
	Status  int
}

type installationAccountInfo struct {
	Login string
	Type  string
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

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
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
		if endedAt, hasEndedAt := inst["access_ended_at"]; hasEndedAt && endedAt != nil {
			continue
		}
		account := installationAccount(inst)
		if strings.ToLower(account.Login) == lowerLogin && strings.EqualFold(account.Type, "User") {
			return inst
		}
	}
	return nil
}

func installationAccount(inst map[string]any) installationAccountInfo {
	account, _ := inst["account"].(map[string]any)
	login, _ := account["login"].(string)
	accountType, _ := account["type"].(string)
	if accountType == "" {
		accountType, _ = inst["target_type"].(string)
	}
	return installationAccountInfo{Login: login, Type: accountType}
}

func mapDeployCreateRepoError(err error, details ...string) *ghError {
	if ghErr, ok := err.(*ghError); ok {
		if ghErr.Status == http.StatusForbidden && strings.Contains(strings.ToLower(ghErr.Message), "resource not accessible by integration") {
			message := "Unable to create repository: GitHub token cannot create repositories in this context."
			if len(details) > 0 {
				message += " " + strings.Join(details, " ")
			}
			message += " Check app installation account, token permissions, and repository owner."
			return &ghError{Message: message, Status: http.StatusBadGateway}
		}
	}
	return &ghError{Message: fmt.Sprintf("Unable to create repository: %v", err), Status: http.StatusBadGateway}
}

func uploadFileToRepo(token, owner, repo, branch, filePath string, content []byte) error {
	parts := strings.Split(filePath, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	encodedPath := strings.Join(parts, "/")

	var existingSha string
	readURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, repo, encodedPath, url.QueryEscape(branch))
	req, err := http.NewRequest(http.MethodGet, readURL, nil)
	if err != nil {
		return err
	}
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
	ThemeRepoLink   string `json:"themeRepoLink"`
}

type parsedThemeRepo struct {
	Repo   string
	Ref    string
	SubDir string // optional subdirectory path within the repo, e.g. "themes/graphic"
}

var githubRepoSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func parseThemeRepoLink(raw string) (parsedThemeRepo, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return parsedThemeRepo{}, fmt.Errorf("themeRepoLink is required")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return parsedThemeRepo{}, fmt.Errorf("themeRepoLink must be a valid GitHub URL")
	}

	host := strings.ToLower(parsed.Hostname())
	if parsed.Scheme != "https" || host != "github.com" {
		return parsedThemeRepo{}, fmt.Errorf("themeRepoLink must use https://github.com")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return parsedThemeRepo{}, fmt.Errorf("themeRepoLink must not include query parameters or fragments")
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return parsedThemeRepo{}, fmt.Errorf("themeRepoLink must point to a GitHub repository")
	}

	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSpace(parts[1])
	if !githubRepoSegmentPattern.MatchString(owner) || !githubRepoSegmentPattern.MatchString(repo) {
		return parsedThemeRepo{}, fmt.Errorf("themeRepoLink contains an invalid owner or repository name")
	}

	ref := "main"
	subDir := ""
	if len(parts) > 2 {
		if len(parts) < 4 || parts[2] != "tree" {
			return parsedThemeRepo{}, fmt.Errorf("themeRepoLink must use repository root or /tree/{ref} format")
		}
		ref = strings.TrimSpace(parts[3])
		if ref == "" || strings.Contains(ref, "..") || !githubRepoSegmentPattern.MatchString(ref) {
			return parsedThemeRepo{}, fmt.Errorf("themeRepoLink contains an invalid ref; ref must be a single path segment with no slashes")
		}
		if len(parts) > 4 {
			for _, seg := range parts[4:] {
				if seg == "" || strings.Contains(seg, "..") || !githubRepoSegmentPattern.MatchString(seg) {
					return parsedThemeRepo{}, fmt.Errorf("themeRepoLink contains an invalid path segment")
				}
			}
			subDir = strings.Join(parts[4:], "/")
		}
	}

	return parsedThemeRepo{Repo: owner + "/" + repo, Ref: ref, SubDir: subDir}, nil
}

type deployResult struct {
	RepositoryURL  string `json:"repositoryUrl"`
	PagesURL       string `json:"pagesUrl"`
	RepoFullName   string `json:"repoFullName"`
	ReusedExisting bool   `json:"reusedExistingRepo"`
	InstallationID any    `json:"installationId,omitempty"`
}

func createRepositoryAndDeployTheme(ctx context.Context, userToken, userLogin string, installation map[string]any, installationID int64, params deployParams) (*deployResult, error) {
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
	if !strings.EqualFold(repositoryOwner, userLogin) {
		return nil, &ghError{Message: "Repository owner must match your personal GitHub account for deploy creation.", Status: http.StatusBadRequest}
	}

	themeSource, err := parseThemeRepoLink(params.ThemeRepoLink)
	if err != nil {
		return nil, &ghError{Message: err.Error(), Status: http.StatusBadRequest}
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

	bundle, err := buildThemeBundle(theme, repositoryOwner, repositoryName, params.ResumeData, themeSource.Repo, themeSource.Ref, themeSource.SubDir, params.ThemeRepoLink)
	if err != nil {
		return nil, &ghError{Message: fmt.Sprintf("Unable to build %s theme bundle: %v", theme, err), Status: http.StatusInternalServerError}
	}
	slog.InfoContext(ctx, "deploy.bundle_ready",
		"request_id", requestIDFromContext(ctx),
		"theme", theme,
		"source", themeSource.Repo+"@"+themeSource.Ref,
		"file_count", len(bundle),
		"repo_owner", repositoryOwner,
		"repo_name", repositoryName,
	)

	instToken, err := getInstallationToken(installationID)
	if err != nil {
		return nil, &ghError{Message: fmt.Sprintf("Unable to obtain bot installation token: %v", err), Status: http.StatusInternalServerError}
	}
	slog.InfoContext(ctx, "deploy.installation_token_ready",
		"request_id", requestIDFromContext(ctx),
		"installation_id", installationID,
		"installation_login", installationAccount(installation).Login,
		"installation_type", installationAccount(installation).Type,
		"repository_selection", instToken.RepositorySelection,
		"permissions_contents", instToken.Permissions["contents"],
		"permissions_administration", instToken.Permissions["administration"],
		"single_file_name", instToken.SingleFileName,
		"expires_at", instToken.ExpiresAt,
	)

	if strings.EqualFold(instToken.RepositorySelection, "selected") {
		return nil, &ghError{Message: "GitHub App installation is limited to selected repositories; repository creation requires broader access.", Status: http.StatusBadRequest}
	}
	if instToken.SingleFileName != "" {
		return nil, &ghError{Message: "GitHub App installation is restricted to a single file and cannot create repositories.", Status: http.StatusBadRequest}
	}
	if permission := instToken.Permissions["contents"]; permission != "write" {
		return nil, &ghError{Message: "GitHub App token does not include contents:write permission for repository creation.", Status: http.StatusBadRequest}
	}

	uploadToken := instToken.Token
	if _, err := getInstallationRepositories(uploadToken); err != nil {
		return nil, &ghError{Message: fmt.Sprintf("Unable to verify installation repository access: %v", err), Status: http.StatusBadGateway}
	}

	reusedExisting := false
	repo, err := ghRequest(uploadToken, fmt.Sprintf("/repos/%s/%s", repositoryOwner, repositoryName), http.MethodGet, nil)
	if err != nil {
		if ghErr, ok := err.(*ghError); ok && ghErr.Status != http.StatusNotFound {
			slog.ErrorContext(ctx, "deploy.repo_lookup_failed",
				"request_id", requestIDFromContext(ctx),
				"repo_owner", repositoryOwner,
				"repo_name", repositoryName,
				"status", ghErr.Status,
				"error", ghErr.Message,
			)
			return nil, &ghError{Message: fmt.Sprintf("Unable to check repository access: %v", err), Status: http.StatusBadGateway}
		}
		slog.InfoContext(ctx, "deploy.repo_not_found_create_start",
			"request_id", requestIDFromContext(ctx),
			"repo_owner", repositoryOwner,
			"repo_name", repositoryName,
		)
		repo, err = ghRequest(userToken, "/user/repos", http.MethodPost, map[string]any{
			"name":         repositoryName,
			"description":  fmt.Sprintf("Portfolio site deployed with the %s theme", theme),
			"private":      params.PrivateRepo,
			"auto_init":    true,
			"has_issues":   false,
			"has_projects": false,
			"has_wiki":     false,
		})
		if err != nil {
			slog.ErrorContext(ctx, "deploy.repo_create_failed",
				"request_id", requestIDFromContext(ctx),
				"repo_owner", repositoryOwner,
				"repo_name", repositoryName,
				"create_token_type", "oauth_user",
				"installation_id", installationID,
				"installation_login", installationAccount(installation).Login,
				"installation_type", installationAccount(installation).Type,
				"repository_selection", instToken.RepositorySelection,
				"permissions_contents", instToken.Permissions["contents"],
				"permissions_administration", instToken.Permissions["administration"],
				"error", err,
			)
			return nil, mapDeployCreateRepoError(err,
				"Ensure your OAuth GitHub session has repository-create capability and the app is installed on your personal account.",
			)
		}
		slog.InfoContext(ctx, "deploy.repo_created",
			"request_id", requestIDFromContext(ctx),
			"repo_owner", repositoryOwner,
			"repo_name", repositoryName,
		)
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
	slog.InfoContext(ctx, "deploy.theme_committed",
		"request_id", requestIDFromContext(ctx),
		"theme", theme,
		"repo_owner", repositoryOwner,
		"repo_name", repositoryName,
		"file_count", len(bundle),
	)

	if _, err := ghRequest(uploadToken, fmt.Sprintf("/repos/%s/%s/pages", repositoryOwner, repositoryName), http.MethodPost, map[string]any{"build_type": "workflow"}); err != nil {
		slog.WarnContext(ctx, "deploy.pages_enable_failed",
			"request_id", requestIDFromContext(ctx),
			"repo_owner", repositoryOwner,
			"repo_name", repositoryName,
			"error", err,
		)
	}

	htmlURL, _ := repo["html_url"].(string)
	fullName, _ := repo["full_name"].(string)

	return &deployResult{
		RepositoryURL:  htmlURL,
		PagesURL:       fmt.Sprintf("https://%s.github.io/%s/", repositoryOwner, repositoryName),
		RepoFullName:   fullName,
		ReusedExisting: reusedExisting,
	}, nil
}
