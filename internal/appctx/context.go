package appctx

import (
	"crypto/rsa"
	"log/slog"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// AppContext holds all application configuration and dependencies
// that were previously stored as package-level globals.
type AppContext struct {
	// HTTP Configuration
	Port       string
	HTTPAddr   string
	BackendDir string
	LogLevel   string

	// OAuth Configuration
	OAuthCallbackURL  string
	OAuthStateCookie  string
	OAuthReturnCookie string
	OAuthTokenCookie  string

	// GitHub App Configuration
	AppID           string
	AppClientID     string
	AppClientSecret string
	AppInstallURL   string
	AppPrivateKey   *rsa.PrivateKey

	// AI Configuration
	GeminiModel  string
	GoogleAPIKey string

	// CORS Configuration
	CORSOrigins     []string
	CORSCredentials bool

	// Resume Validation Schema
	ResumeSchema *jsonschema.Schema

	// Shared Assets Configuration
	SharedAssetsRepo string
	SharedAssetsRef  string

	// Logger
	Logger *slog.Logger
}

// OAuth Cookie Names
const (
	OAuthStateCookieName  = "gh_oauth_state"
	OAuthReturnCookieName = "gh_oauth_return"
	OAuthTokenCookieName  = "gh_access_token"

	// SharedAssetsRepo and SharedAssetsRef define the fixed source for shared
	// runtime files (e.g src/rxresume.js) that are independent of the theme
	// repository link provided by the caller at deploy time.
	SharedAssetsRepo = "oneclick-portfolio/awesome-github-portfolio"
	SharedAssetsRef  = "main"
)
