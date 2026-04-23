package utils

import (
	"os"
	"strings"
)

func IsProduction() bool {
	return os.Getenv("NODE_ENV") == "production"
}

func ParseCSVEnv(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		result = append(result, v)
	}
	return result
}
