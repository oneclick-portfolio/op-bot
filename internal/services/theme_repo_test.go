package services

import "testing"

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
			got, err := ParseThemeRepoLink(tt.input)
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
