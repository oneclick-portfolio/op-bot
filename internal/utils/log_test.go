package utils

import (
	"log/slog"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   slog.Level
		wantOK bool
	}{
		{name: "empty defaults to info", input: "", want: slog.LevelInfo, wantOK: true},
		{name: "info", input: "info", want: slog.LevelInfo, wantOK: true},
		{name: "debug", input: "debug", want: slog.LevelDebug, wantOK: true},
		{name: "warn", input: "warn", want: slog.LevelWarn, wantOK: true},
		{name: "warning", input: "warning", want: slog.LevelWarn, wantOK: true},
		{name: "error", input: "error", want: slog.LevelError, wantOK: true},
		{name: "mixed case trimmed", input: " WARN ", want: slog.LevelWarn, wantOK: true},
		{name: "invalid falls back to info", input: "verbose", want: slog.LevelInfo, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseLogLevel(tt.input)
			if got != tt.want {
				t.Fatalf("ParseLogLevel(%q) level = %v, want %v", tt.input, got, tt.want)
			}
			if ok != tt.wantOK {
				t.Fatalf("ParseLogLevel(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
		})
	}
}
