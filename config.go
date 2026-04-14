package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

var (
	backendDir       string
	port             string
	httpAddr         string
	appClientID      string
	appClientSecret  string
	geminiModel      string
	appInstallURL    string
	oauthCallbackURL string
	corsOrigins      []string
	corsCredentials  bool
	resumeSchema     *jsonschema.Schema
	resumeSchemaOnce sync.Once
	resumeSchemaErr  error
	appID            string
	appPrivateKey    *rsa.PrivateKey
	googleAPIKey     string
	logLevel         string
)

const (
	oauthStateCookie  = "gh_oauth_state"
	oauthReturnCookie = "gh_oauth_return"
	oauthTokenCookie  = "gh_access_token"

	// sharedAssetsRepo and sharedAssetsRef define the fixed source for shared
	// runtime files (e.g. src/rxresume.js) that are independent of the theme
	// repository link provided by the caller at deploy time.
	sharedAssetsRepo = "oneclick-portfolio/awesome-github-portfolio"
	sharedAssetsRef  = "main"
)

var themeFiles = map[string]map[string]string{
	"modern": {
		"index": "themes/modern/index.html",
		"app":   "themes/modern/app.js",
		"style": "themes/modern/styles.css",
	},
	"graphic": {
		"index": "themes/graphic/index.html",
		"app":   "themes/graphic/app.js",
		"style": "themes/graphic/style.css",
	},
	"newspaper": {
		"index": "themes/newspaper/index.html",
		"app":   "themes/newspaper/app.js",
		"style": "themes/newspaper/style.css",
	},
	"vscode": {
		"index": "themes/vscode/index.html",
		"app":   "themes/vscode/app.js",
		"style": "themes/vscode/style.css",
	},
}

func init() {
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	backendDir = exeDir

	loadDotEnv(backendDir, exeDir)

	port = strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	appClientID = strings.TrimSpace(os.Getenv("APP_CLIENT_ID"))
	appClientSecret = strings.TrimSpace(os.Getenv("APP_CLIENT_SECRET"))
	appInstallURL = normalizeInstallURL(strings.TrimSpace(os.Getenv("APP_INSTALL_URL")))
	oauthCallbackURL = strings.TrimSpace(os.Getenv("OAUTH_CALLBACK_URL"))
	appID = strings.TrimSpace(os.Getenv("APP_ID"))
	googleAPIKey = strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
	geminiModel = strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
	if geminiModel == "" {
		geminiModel = "gemini-2.0-flash"
	}
	logLevel = strings.TrimSpace(os.Getenv("LOG_LEVEL"))
	if pkPEM := strings.TrimSpace(os.Getenv("APP_PRIVATE_KEY")); pkPEM != "" {
		pkPEM = strings.ReplaceAll(pkPEM, `\n`, "\n")
		if block, _ := pem.Decode([]byte(pkPEM)); block != nil {
			if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
				appPrivateKey = key
			}
		}
	}
	if oauthCallbackURL == "" && !isProduction() {
		oauthCallbackURL = "http://localhost:" + port + "/auth/github/callback"
	}
	httpAddr = ":" + port

	corsOrigins = parseCSVEnv("CORS_ALLOWED_ORIGINS")
	if len(corsOrigins) == 0 && !isProduction() {
		corsOrigins = []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
			"http://localhost:4173",
			"http://127.0.0.1:4173",
		}
	}
	corsCredentials = true
	for _, origin := range corsOrigins {
		if origin == "*" {
			corsCredentials = false
			break
		}
	}
}

func normalizeInstallURL(u string) string {
	if u == "" {
		return ""
	}
	if regexp.MustCompile(`/installations/`).MatchString(u) {
		return u
	}
	if matched, _ := regexp.MatchString(`^https://github\.com/apps/[^/]+/?$`, u); matched {
		return strings.TrimSuffix(u, "/") + "/installations/new"
	}
	return u
}

func validateTheme(theme string) bool {
	_, ok := themeFiles[theme]
	return ok
}

func normalizeRepoName(name string) string {
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

func getThemeLabel(theme string) string {
	if len(theme) == 0 {
		return ""
	}
	return strings.ToUpper(theme[:1]) + theme[1:]
}

func isProduction() bool {
	return os.Getenv("NODE_ENV") == "production"
}

func loadDotEnv(dirs ...string) {
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

func parseCSVEnv(key string) []string {
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
