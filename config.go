package main

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"op-bot/internal/appctx"
	"op-bot/internal/utils"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Package-level globals maintained for backward compatibility during migration.
// These will be removed in Phase 5 as individual packages are updated to use AppContext.
var (
	backendDir       string
	appPrivateKey    *rsa.PrivateKey
	appID            string
	appInstallURL    string
	appClientID      string
	appClientSecret  string
	port             string
	httpAddr         string
	oauthCallbackURL string
	googleAPIKey     string
	geminiModel      string
	corsOrigins      []string
	corsCredentials  bool
	resumeSchema     *jsonschema.Schema
	resumeSchemaOnce sync.Once
	resumeSchemaErr  error
	sessionManager   *authSessionManager
)

// OAuth Cookie Names - constants for backwards compatibility
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

// LoadAppContext loads environment variables and returns an initialized AppContext.
// This also populates package-level globals for backward compatibility during migration.
func LoadAppContext() *appctx.AppContext {
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	backendDir := exeDir

	utils.LoadDotEnv(backendDir, exeDir)

	// Load basic HTTP configuration
	port = strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	httpAddr = ":" + port

	// Load OAuth configuration
	oauthCallbackURL = strings.TrimSpace(os.Getenv("OAUTH_CALLBACK_URL"))
	if oauthCallbackURL == "" && !utils.IsProduction() {
		oauthCallbackURL = "http://localhost:" + port + "/auth/github/callback"
	}

	// Load CORS configuration
	corsOrigins = utils.ParseCSVEnv("CORS_ALLOWED_ORIGINS")
	if len(corsOrigins) == 0 && !utils.IsProduction() {
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

	// Load GitHub App configuration
	appClientID = strings.TrimSpace(os.Getenv("APP_CLIENT_ID"))
	appClientSecret = strings.TrimSpace(os.Getenv("APP_CLIENT_SECRET"))
	appInstallURL = strings.TrimSpace(os.Getenv("APP_INSTALL_URL"))
	appID = strings.TrimSpace(os.Getenv("APP_ID"))
	googleAPIKey = strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))

	// Load AI configuration
	geminiModel = strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
	if geminiModel == "" {
		geminiModel = "gemini-3.1-flash-lite-preview"
	}

	// Load logging configuration
	logLevel := strings.TrimSpace(os.Getenv("LOG_LEVEL"))

	// Load auth session configuration
	authSessionTTL := 4 * time.Hour
	if rawTTL := strings.TrimSpace(os.Getenv("AUTH_SESSION_TTL_SECONDS")); rawTTL != "" {
		if ttlSeconds, err := strconv.Atoi(rawTTL); err == nil && ttlSeconds > 0 {
			authSessionTTL = time.Duration(ttlSeconds) * time.Second
		}
	}
	sessionManager = newAuthSessionManager(authSessionTTL)

	// Parse GitHub App private key
	if pkPEM := strings.TrimSpace(os.Getenv("APP_PRIVATE_KEY")); pkPEM != "" {
		pkPEM = strings.ReplaceAll(pkPEM, `\n`, "\n")
		if block, _ := pem.Decode([]byte(pkPEM)); block != nil {
			if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
				appPrivateKey = key
			}
		}
	}

	// Load resume schema
	schemaPath := filepath.Join(backendDir, "schema", "v5.json")
	if data, err := os.ReadFile(schemaPath); err == nil {
		if schema, err := jsonschema.UnmarshalJSON(bytes.NewReader(data)); err == nil {
			resumeSchema = schema.(*jsonschema.Schema)
		}
	}

	return &appctx.AppContext{
		Port:              port,
		HTTPAddr:          httpAddr,
		BackendDir:        backendDir,
		LogLevel:          logLevel,
		OAuthCallbackURL:  oauthCallbackURL,
		OAuthStateCookie:  appctx.OAuthStateCookieName,
		OAuthReturnCookie: appctx.OAuthReturnCookieName,
		OAuthTokenCookie:  appctx.OAuthTokenCookieName,
		AppID:             appID,
		AppClientID:       appClientID,
		AppClientSecret:   appClientSecret,
		AppInstallURL:     appInstallURL,
		AppPrivateKey:     appPrivateKey,
		GeminiModel:       geminiModel,
		GoogleAPIKey:      googleAPIKey,
		CORSOrigins:       corsOrigins,
		CORSCredentials:   corsCredentials,
		ResumeSchema:      resumeSchema,
		SharedAssetsRepo:  appctx.SharedAssetsRepo,
		SharedAssetsRef:   appctx.SharedAssetsRef,
	}
}
