package models

import (
	"encoding/json"
	"testing"
)

func TestDeployParamsJSONRoundTrip(t *testing.T) {
	in := DeployParams{
		Theme:          "modern",
		RepositoryName: "portfolio",
		PrivateRepo:    true,
		ThemeRepoLink:  "https://github.com/org/repo",
	}

	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var out DeployParams
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.RepositoryName != in.RepositoryName || out.Theme != in.Theme {
		t.Fatalf("round trip mismatch: %#v vs %#v", out, in)
	}
}

func TestDeployResultInstallationIDOptional(t *testing.T) {
	out := DeployResult{RepositoryURL: "https://github.com/org/repo"}
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(data) == "{}" {
		t.Fatalf("expected repositoryUrl to be present, got %s", string(data))
	}
}
