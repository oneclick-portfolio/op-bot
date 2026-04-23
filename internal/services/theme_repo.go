package services

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"op-bot/internal/models"
)

var githubRepoSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func ParseThemeRepoLink(raw string) (models.ParsedThemeRepo, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return models.ParsedThemeRepo{}, fmt.Errorf("themeRepoLink is required")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return models.ParsedThemeRepo{}, fmt.Errorf("themeRepoLink must be a valid GitHub URL")
	}

	host := strings.ToLower(parsed.Hostname())
	if parsed.Scheme != "https" || host != "github.com" {
		return models.ParsedThemeRepo{}, fmt.Errorf("themeRepoLink must use https://github.com")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return models.ParsedThemeRepo{}, fmt.Errorf("themeRepoLink must not include query parameters or fragments")
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return models.ParsedThemeRepo{}, fmt.Errorf("themeRepoLink must point to a GitHub repository")
	}

	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSpace(parts[1])
	if !githubRepoSegmentPattern.MatchString(owner) || !githubRepoSegmentPattern.MatchString(repo) {
		return models.ParsedThemeRepo{}, fmt.Errorf("themeRepoLink contains an invalid owner or repository name")
	}

	ref := "main"
	subDir := ""
	if len(parts) > 2 {
		if len(parts) < 4 || parts[2] != "tree" {
			return models.ParsedThemeRepo{}, fmt.Errorf("themeRepoLink must use repository root or /tree/{ref} format")
		}
		ref = strings.TrimSpace(parts[3])
		if ref == "" || strings.Contains(ref, "..") || !githubRepoSegmentPattern.MatchString(ref) {
			return models.ParsedThemeRepo{}, fmt.Errorf("themeRepoLink contains an invalid ref; ref must be a single path segment with no slashes")
		}
		if len(parts) > 4 {
			for _, seg := range parts[4:] {
				if seg == "" || strings.Contains(seg, "..") || !githubRepoSegmentPattern.MatchString(seg) {
					return models.ParsedThemeRepo{}, fmt.Errorf("themeRepoLink contains an invalid path segment")
				}
			}
			subDir = strings.Join(parts[4:], "/")
		}
	}

	return models.ParsedThemeRepo{Repo: owner + "/" + repo, Ref: ref, SubDir: subDir}, nil
}
