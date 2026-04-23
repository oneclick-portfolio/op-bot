package utils

import "testing"

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
		result := NormalizeRepoName(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeRepoName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGetThemeLabel(t *testing.T) {
	if got := GetThemeLabel("modern"); got != "Modern" {
		t.Errorf("GetThemeLabel(modern) = %q, want %q", got, "Modern")
	}
	if got := GetThemeLabel(""); got != "" {
		t.Errorf("GetThemeLabel('') = %q, want ''", got)
	}
}
