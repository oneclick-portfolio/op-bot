package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	const key = "DOTENV_TEST_VALUE"
	old := os.Getenv(key)
	defer os.Setenv(key, old)
	_ = os.Unsetenv(key)

	dir := t.TempDir()
	content := key + "=loaded\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	LoadDotEnv(dir)
	if got := os.Getenv(key); got != "loaded" {
		t.Fatalf("env %s = %q, want %q", key, got, "loaded")
	}
}
