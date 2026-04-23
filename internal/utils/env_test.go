package utils

import (
	"os"
	"reflect"
	"testing"
)

func TestIsProduction(t *testing.T) {
	old := os.Getenv("NODE_ENV")
	defer os.Setenv("NODE_ENV", old)

	_ = os.Setenv("NODE_ENV", "production")
	if !IsProduction() {
		t.Fatal("expected IsProduction() to return true")
	}

	_ = os.Setenv("NODE_ENV", "development")
	if IsProduction() {
		t.Fatal("expected IsProduction() to return false")
	}
}

func TestParseCSVEnv(t *testing.T) {
	const key = "TEST_CSV_ENV"
	old := os.Getenv(key)
	defer os.Setenv(key, old)

	_ = os.Setenv(key, "")
	if got := ParseCSVEnv(key); got != nil {
		t.Fatalf("expected nil for empty env, got %#v", got)
	}

	_ = os.Setenv(key, " a, b ,, c ")
	want := []string{"a", "b", "c"}
	if got := ParseCSVEnv(key); !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseCSVEnv() = %#v, want %#v", got, want)
	}
}
