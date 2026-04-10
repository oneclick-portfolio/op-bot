package main

import (
	"bytes"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
)

//go:embed docs/swagger.json
var openapiSpec []byte

func githubInstallHint() string {
	if appInstallURL != "" {
		return fmt.Sprintf("Install the app first: %s", appInstallURL)
	}
	return "Install the GitHub App for your account/repository and try again."
}

func setCookie(w http.ResponseWriter, name, value string, maxAge int, secure bool) {
	sameSite := http.SameSiteLaxMode
	if secure {
		// In production the frontend (oneclick-portfolio.github.io) and backend
		// (vercel.app) are on different top-level domains. SameSite=None;Secure
		// is required so cross-site fetch requests include the cookie.
		sameSite = http.SameSiteNoneMode
	}
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: sameSite,
		Secure:   secure,
	}
	http.SetCookie(w, cookie)
}

func clearCookie(w http.ResponseWriter, name string) {
	setCookie(w, name, "", -1, false)
}

func getCookie(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func requireGitHubAppConfig(w http.ResponseWriter) bool {
	if appClientID == "" || appClientSecret == "" {
		writeError(w, http.StatusInternalServerError, "GitHub App auth is not configured. Set APP_CLIENT_ID and APP_CLIENT_SECRET.")
		return false
	}
	return true
}

// @Summary Start GitHub OAuth flow
// @Description Redirects to GitHub for OAuth authorization
// @Tags auth
// @Param returnTo query string false "URL to return to after auth"
// @Success 302
// @Failure 500 {object} apiErrorResponse
// @Router /auth/github/start [get]
func handleAuthGitHubStart(w http.ResponseWriter, r *http.Request) {
	if !requireGitHubAppConfig(w) {
		return
	}

	stateBytes := make([]byte, 18)
	if _, err := rand.Read(stateBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "Unable to initialize OAuth flow.")
		return
	}
	state := hex.EncodeToString(stateBytes)

	returnTo := r.URL.Query().Get("returnTo")
	if returnTo == "" {
		returnTo = "/"
	}

	setCookie(w, oauthStateCookie, state, 600, isProduction())
	setCookie(w, oauthReturnCookie, returnTo, 600, isProduction())

	authURL, err := url.Parse("https://github.com/login/oauth/authorize")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Unable to initialize OAuth flow.")
		return
	}
	q := authURL.Query()
	q.Set("client_id", appClientID)
	q.Set("state", state)
	q.Set("allow_signup", "true")
	if oauthCallbackURL != "" {
		q.Set("redirect_uri", oauthCallbackURL)
	}
	authURL.RawQuery = q.Encode()

	http.Redirect(w, r, authURL.String(), http.StatusFound)
}

// @Summary GitHub OAuth callback
// @Description Handles OAuth callback from GitHub and sets auth cookie
// @Tags auth
// @Param code query string true "OAuth code"
// @Param state query string true "OAuth state"
// @Success 302
// @Failure 400 {object} apiErrorResponse "Invalid OAuth state"
// @Failure 500 {object} apiErrorResponse "OAuth callback failed"
// @Router /auth/github/callback [get]
func handleAuthGitHubCallback(w http.ResponseWriter, r *http.Request) {
	if !requireGitHubAppConfig(w) {
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	setupAction := r.URL.Query().Get("setup_action")
	installationID := r.URL.Query().Get("installation_id")

	savedState := getCookie(r, oauthStateCookie)
	returnTo := getCookie(r, oauthReturnCookie)
	if returnTo == "" {
		returnTo = "/"
	}

	clearCookie(w, oauthStateCookie)
	clearCookie(w, oauthReturnCookie)

	isInstallCallback := installationID != "" || setupAction == "install" || setupAction == "update"
	hasValidState := code != "" && state != "" && savedState != "" && state == savedState
	allowStatelessInstallCallback := code != "" && isInstallCallback && savedState == ""

	if !hasValidState && !allowStatelessInstallCallback {
		writeError(w, http.StatusBadRequest, "Invalid OAuth state. Please try connecting again.")
		return
	}

	tokenPayload := map[string]string{
		"client_id":     appClientID,
		"client_secret": appClientSecret,
		"code":          code,
		"state":         state,
	}
	if oauthCallbackURL != "" {
		tokenPayload["redirect_uri"] = oauthCallbackURL
	}
	tokenBody, err := json.Marshal(tokenPayload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "OAuth callback failed.")
		return
	}

	req, err := http.NewRequest(http.MethodPost, "https://github.com/login/oauth/access_token", bytes.NewReader(tokenBody))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "OAuth callback failed.")
		return
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "OAuth callback failed.", err.Error())
		return
	}
	defer resp.Body.Close()

	var tokenResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to parse token response")
		return
	}

	if tokenResp["error"] != nil || tokenResp["access_token"] == nil {
		msg := "GitHub App user token exchange failed."
		if desc, ok := tokenResp["error_description"].(string); ok {
			msg = desc
		} else if errStr, ok := tokenResp["error"].(string); ok {
			msg = errStr
		}
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	userToken, _ := tokenResp["access_token"].(string)
	setCookie(w, oauthTokenCookie, userToken, 60*60*4, isProduction())

	redirectURL, err := url.Parse(returnTo)
	if err != nil {
		redirectURL = &url.URL{Path: "/"}
	}
	q := redirectURL.Query()
	q.Set("connected", "1")
	redirectURL.RawQuery = q.Encode()

	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

// @Summary Get current GitHub user
// @Description Returns the authenticated GitHub user and app installation info
// @Tags github
// @Produce json
// @Success 200 {object} MeResponse
// @Failure 401 {object} apiErrorResponse
// @Router /api/github/me [get]
func handleAPIGitHubMe(w http.ResponseWriter, r *http.Request) {
	token := getCookie(r, oauthTokenCookie)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "Not connected")
		return
	}

	user, err := getGitHubUserFromToken(token)
	if err != nil {
		if ghErr, ok := err.(*ghError); ok && ghErr.Status == http.StatusUnauthorized {
			clearCookie(w, oauthTokenCookie)
		}
		writeError(w, http.StatusUnauthorized, "GitHub session expired. Please reconnect.")
		return
	}

	installations, err := getGitHubInstallationsFromToken(token)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("Unable to load GitHub App installations: %v", err))
		return
	}
	login, _ := user["login"].(string)
	chosenInstallation := pickInstallationForUser(installations, login)

	var installationID any
	if chosenInstallation != nil {
		installationID = chosenInstallation["id"]
	}

	var installURL any
	if appInstallURL != "" {
		installURL = appInstallURL
	}

	var repositorySelection any
	if chosenInstallation != nil {
		repositorySelection = chosenInstallation["repository_selection"]
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"login":     user["login"],
			"name":      user["name"],
			"avatarUrl": user["avatar_url"],
		},
		"githubApp": map[string]any{
			"installed":           chosenInstallation != nil,
			"installationId":      installationID,
			"installUrl":          installURL,
			"repositorySelection": repositorySelection,
		},
	})
}

// @Summary Get current GitHub installations repositories
// @Description Returns the repositories the GitHub app has access to
// @Tags github
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} apiErrorResponse
// @Failure 502 {object} apiErrorResponse "Unable to load repositories"
// @Router /api/github/repos [get]
func handleAPIGitHubRepos(w http.ResponseWriter, r *http.Request) {
	token := getCookie(r, oauthTokenCookie)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "Not connected")
		return
	}

	user, err := getGitHubUserFromToken(token)
	if err != nil {
		if ghErr, ok := err.(*ghError); ok && ghErr.Status == http.StatusUnauthorized {
			clearCookie(w, oauthTokenCookie)
		}
		writeError(w, http.StatusUnauthorized, "GitHub session expired. Please reconnect.")
		return
	}

	installations, err := getGitHubInstallationsFromToken(token)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("Unable to load GitHub App installations: %v", err))
		return
	}
	login, _ := user["login"].(string)
	chosenInstallation := pickInstallationForUser(installations, login)
	if chosenInstallation == nil {
		writeError(w, http.StatusBadRequest, "GitHub App is not installed for this account.", githubInstallHint())
		return
	}

	var installationID int64
	if id, ok := chosenInstallation["id"].(float64); ok {
		installationID = int64(id)
	}
	if installationID == 0 {
		writeError(w, http.StatusBadGateway, "Unable to resolve GitHub App installation id for this account.")
		return
	}
	instToken, err := getInstallationToken(installationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Unable to obtain bot installation token", err.Error())
		return
	}

	repos, err := getInstallationRepositories(instToken.Token)
	if err != nil {
		code := http.StatusBadGateway
		if ghErr, ok := err.(*ghError); ok && ghErr.Status != 0 {
			code = ghErr.Status
		}
		writeError(w, code, "Unable to fetch installation repositories")
		return
	}

	type repoInfo struct {
		Name          string `json:"name"`
		FullName      string `json:"fullName"`
		Owner         string `json:"owner"`
		Private       bool   `json:"private"`
		DefaultBranch string `json:"defaultBranch"`
		HTMLURL       string `json:"htmlUrl"`
	}
	result := make([]repoInfo, 0, len(repos))
	for _, repo := range repos {
		owner := ""
		if ownerObj, ok := repo["owner"].(map[string]any); ok {
			owner, _ = ownerObj["login"].(string)
		}
		name, _ := repo["name"].(string)
		fullName, _ := repo["full_name"].(string)
		privateRepo, _ := repo["private"].(bool)
		defaultBranch, _ := repo["default_branch"].(string)
		htmlURL, _ := repo["html_url"].(string)
		result = append(result, repoInfo{
			Name:          name,
			FullName:      fullName,
			Owner:         owner,
			Private:       privateRepo,
			DefaultBranch: defaultBranch,
			HTMLURL:       htmlURL,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"installationId": chosenInstallation["id"],
		"repositories":   result,
	})
}

// @Summary Logout from GitHub
// @Description Clears the GitHub auth cookie
// @Tags github
// @Success 204 "No Content"
// @Router /api/github/logout [post]
func handleAPIGitHubLogout(w http.ResponseWriter, r *http.Request) {
	clearCookie(w, oauthTokenCookie)
	w.WriteHeader(http.StatusNoContent)
}

// @Summary Validate resume JSON
// @Description Validates resume data against the RxResume schema
// @Tags resume
// @Accept json
// @Produce json
// @Param request body ValidateRequest true "Resume JSON data to validate"
// @Success 200 {object} ValidationResult
// @Failure 400 {object} apiErrorResponse "Bad Request"
// @Router /api/resume/validate [post]
func handleAPIResumeValidate(w http.ResponseWriter, r *http.Request) {
	slog.InfoContext(r.Context(), "resume.validate.start",
		"request_id", requestIDFromContext(r.Context()),
		"remote_hash", redactRemoteAddr(r.RemoteAddr),
	)

	var body struct {
		ResumeData any `json:"resumeData"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		slog.WarnContext(r.Context(), "resume.validate.invalid_json",
			"request_id", requestIDFromContext(r.Context()),
			"remote_hash", redactRemoteAddr(r.RemoteAddr),
			"error", err,
		)
		writeError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	result := validateResumeData(body.ResumeData)
	if !result.Valid {
		slog.WarnContext(r.Context(), "resume.validate.failed",
			"request_id", requestIDFromContext(r.Context()),
			"remote_hash", redactRemoteAddr(r.RemoteAddr),
			"error_count", len(result.Errors),
		)
		writeError(w, http.StatusBadRequest, "Resume schema validation failed", result.Errors...)
		return
	}
	slog.InfoContext(r.Context(), "resume.validate.success",
		"request_id", requestIDFromContext(r.Context()),
		"remote_hash", redactRemoteAddr(r.RemoteAddr),
	)
	writeJSON(w, http.StatusOK, result)
}

// @Summary Deploy portfolio theme
// @Description Creates a GitHub repository and deploys the selected theme
// @Tags github
// @Accept json
// @Produce json
// @Param request body deployParams true "Deploy parameters"
// @Success 200 {object} deployResult
// @Failure 400 {object} apiErrorResponse "Bad Request"
// @Failure 401 {object} apiErrorResponse "Unauthorized"
// @Router /api/github/deploy [post]
func handleAPIGitHubDeploy(w http.ResponseWriter, r *http.Request) {
	token := getCookie(r, oauthTokenCookie)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "Connect GitHub first.")
		return
	}

	var params deployParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	me, err := getGitHubUserFromToken(token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "GitHub session expired.")
		return
	}

	installations, err := getGitHubInstallationsFromToken(token)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("Unable to load GitHub App installations: %v", err))
		return
	}
	login, _ := me["login"].(string)
	chosenInstallation := pickInstallationForUser(installations, login)

	if chosenInstallation == nil {
		writeError(w, http.StatusBadRequest, "GitHub App is not installed for this account.", githubInstallHint())
		return
	}

	var installationID int64
	if id, ok := chosenInstallation["id"].(float64); ok {
		installationID = int64(id)
	}
	if installationID == 0 {
		writeError(w, http.StatusBadGateway, "Unable to resolve GitHub App installation id for this account.")
		return
	}

	result, err := createRepositoryAndDeployTheme(r.Context(), token, login, chosenInstallation, installationID, params)
	if err != nil {
		code := http.StatusBadRequest
		if ghErr, ok := err.(*ghError); ok && ghErr.Status != 0 {
			code = ghErr.Status
		}
		writeError(w, code, err.Error())
		return
	}

	result.InstallationID = chosenInstallation["id"]
	writeJSON(w, http.StatusOK, result)
}

func handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
  <title>Op-Bot API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: "/swagger/openapi.json",
      dom_id: '#swagger-ui',
    });
  </script>
</body>
</html>`))
}

func handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(openapiSpec)
}

