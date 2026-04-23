package repository

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type APIError struct {
	Message string
	Status  int
}

func (e *APIError) Error() string {
	return e.Message
}

type GitHubClient struct {
	httpClient *http.Client
}

func NewGitHubClient(httpClient *http.Client) *GitHubClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &GitHubClient{httpClient: httpClient}
}

func (c *GitHubClient) Request(token, endpoint, method string, body any) (map[string]any, error) {
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

	resp, err := c.httpClient.Do(req)
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
		return nil, &APIError{Message: msg, Status: resp.StatusCode}
	}

	return payload, nil
}

func (c *GitHubClient) GetGitHubUserFromToken(token string) (map[string]any, error) {
	return c.Request(token, "/user", http.MethodGet, nil)
}

func (c *GitHubClient) GetGitHubInstallationsFromToken(token string) ([]map[string]any, error) {
	payload, err := c.Request(token, "/user/installations", http.MethodGet, nil)
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

func (c *GitHubClient) GetInstallationRepositories(token string) ([]map[string]any, error) {
	payload, err := c.Request(token, "/installation/repositories?per_page=100", http.MethodGet, nil)
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
