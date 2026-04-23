package utils

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func LoadDotEnv(dirs ...string) {
	loaded := map[string]struct{}{}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		candidates := []string{
			filepath.Join(dir, ".env"),
			filepath.Join(dir, "..", ".env"),
		}
		for _, path := range candidates {
			if _, seen := loaded[path]; seen {
				continue
			}
			if _, err := os.Stat(path); err != nil {
				continue
			}
			_ = godotenv.Load(path)
			loaded[path] = struct{}{}
		}
	}
}
