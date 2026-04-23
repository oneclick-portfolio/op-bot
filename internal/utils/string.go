package utils

import (
	"regexp"
	"strings"
)

func NormalizeRepoName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	re := regexp.MustCompile(`\s+`)
	name = re.ReplaceAllString(name, "-")
	re = regexp.MustCompile(`[^a-z0-9._-]`)
	name = re.ReplaceAllString(name, "-")
	re = regexp.MustCompile(`-+`)
	name = re.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if len(name) > 100 {
		name = name[:100]
	}
	return name
}

func GetThemeLabel(theme string) string {
	if len(theme) == 0 {
		return ""
	}
	return strings.ToUpper(theme[:1]) + theme[1:]
}
