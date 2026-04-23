package services

import (
	"encoding/json"
	"testing"
)

func TestBuildGitHubMeResponse(t *testing.T) {
	user := map[string]any{"login": "alice", "name": "Alice", "avatar_url": "https://example.com/a.png"}
	inst := map[string]any{"id": float64(123), "repository_selection": "all"}

	payload := BuildGitHubMeResponse(user, inst, "https://github.com/apps/test-app")
	ghApp := payload["githubApp"].(map[string]any)
	if installed, _ := ghApp["installed"].(bool); !installed {
		t.Fatalf("installed should be true")
	}
	if ghApp["installationId"] != inst["id"] {
		t.Fatalf("installationId mismatch")
	}
}

func TestBuildGitHubReposResponse(t *testing.T) {
	repos := []map[string]any{{
		"name":           "repo1",
		"full_name":      "alice/repo1",
		"private":        true,
		"default_branch": "main",
		"html_url":       "https://github.com/alice/repo1",
		"owner":          map[string]any{"login": "alice"},
	}}

	payload := BuildGitHubReposResponse(float64(321), repos)
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if got["installationId"] != float64(321) {
		t.Fatalf("installationId mismatch")
	}
	repoItems, ok := got["repositories"].([]any)
	if !ok || len(repoItems) != 1 {
		t.Fatalf("repositories payload malformed: %#v", got["repositories"])
	}
}
