package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"op-bot/internal/models"
	"op-bot/internal/repository"
	"op-bot/internal/services"
	"op-bot/internal/utils"
	"strings"
	"time"
)

type ghError = repository.APIError

type installationAccountInfo struct {
	Login string
	Type  string
}

var githubClient = repository.NewGitHubClient(nil)

func ghRequest(token, endpoint, method string, body any) (map[string]any, error) {
	return githubClient.Request(token, endpoint, method, body)
}

func getGitHubUserFromToken(token string) (map[string]any, error) {
	return githubClient.GetGitHubUserFromToken(token)
}

func getGitHubInstallationsFromToken(token string) ([]map[string]any, error) {
	return githubClient.GetGitHubInstallationsFromToken(token)
}

func getInstallationRepositories(token string) ([]map[string]any, error) {
	return githubClient.GetInstallationRepositories(token)
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

func createRepositoryAndDeployTheme(ctx context.Context, userToken, userLogin string, installation map[string]any, installationID int64, params models.DeployParams) (*models.DeployResult, error) {
	theme := params.Theme
	repositoryName := utils.NormalizeRepoName(params.RepositoryName)
	repositoryOwner := strings.TrimSpace(params.RepositoryOwner)
	if repositoryOwner == "" {
		repositoryOwner = userLogin
	}

	if repositoryName == "" {
		return nil, &ghError{Message: "Repository name is required.", Status: http.StatusBadRequest}
	}
	if !strings.EqualFold(repositoryOwner, userLogin) {
		return nil, &ghError{Message: "Repository owner must match your personal GitHub account for deploy creation.", Status: http.StatusBadRequest}
	}

	themeSource, err := services.ParseThemeRepoLink(params.ThemeRepoLink)
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

	pagesURL := fmt.Sprintf("https://%s.github.io/%s/", repositoryOwner, repositoryName)
	aboutPatch := map[string]any{}
	if params.Description != "" {
		aboutPatch["description"] = params.Description
	}
	switch {
	case params.UseGitHubPagesURL:
		aboutPatch["homepage"] = pagesURL
	case params.HomepageURL != "":
		aboutPatch["homepage"] = params.HomepageURL
	}
	if len(aboutPatch) > 0 {
		if _, err := ghRequest(userToken, fmt.Sprintf("/repos/%s/%s", repositoryOwner, repositoryName), http.MethodPatch, aboutPatch); err != nil {
			slog.WarnContext(ctx, "deploy.about_update_failed",
				"request_id", requestIDFromContext(ctx),
				"repo_owner", repositoryOwner,
				"repo_name", repositoryName,
				"error", err,
			)
		}
	}

	htmlURL, _ := repo["html_url"].(string)
	fullName, _ := repo["full_name"].(string)

	return &models.DeployResult{
		RepositoryURL:  htmlURL,
		PagesURL:       pagesURL,
		RepoFullName:   fullName,
		ReusedExisting: reusedExisting,
	}, nil
}
