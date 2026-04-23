package models

import "testing"

func TestInstallationTokenFields(t *testing.T) {
	tok := InstallationToken{
		Token:       "abc",
		Permissions: map[string]string{"contents": "write"},
	}

	if tok.Token != "abc" {
		t.Fatalf("Token = %q, want %q", tok.Token, "abc")
	}
	if tok.Permissions["contents"] != "write" {
		t.Fatalf("Permissions contents = %q, want %q", tok.Permissions["contents"], "write")
	}
}
