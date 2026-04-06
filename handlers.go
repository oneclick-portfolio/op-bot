package main

import (
	"bytes"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
)

//go:embed openapi.json
var openapiSpec []byte

func setCookie(w http.ResponseWriter, name, value string, maxAge int, secure bool) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
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

func handleAuthGitHubStart(w http.ResponseWriter, r *http.Request) {
	if !requireGitHubAppConfig(w) {
		return
	}

	stateBytes := make([]byte, 18)
	_, _ = rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)

	returnTo := r.URL.Query().Get("returnTo")
	if returnTo == "" {
		returnTo = "/"
	}

	setCookie(w, oauthStateCookie, state, 600, isProduction())
	setCookie(w, oauthReturnCookie, returnTo, 600, isProduction())

	authURL, _ := url.Parse("https://github.com/login/oauth/authorize")
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
	tokenBody, _ := json.Marshal(tokenPayload)

	req, _ := http.NewRequest(http.MethodPost, "https://github.com/login/oauth/access_token", bytes.NewReader(tokenBody))
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

	redirectURL, _ := url.Parse(returnTo)
	q := redirectURL.Query()
	q.Set("connected", "1")
	redirectURL.RawQuery = q.Encode()

	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

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

	installations, _ := getGitHubInstallationsFromToken(token)
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

func handleAPIGitHubLogout(w http.ResponseWriter, r *http.Request) {
	clearCookie(w, oauthTokenCookie)
	w.WriteHeader(http.StatusNoContent)
}

func handleAPIResumeValidate(w http.ResponseWriter, r *http.Request) {
	log.Printf("resume.validate start remote=%s", r.RemoteAddr)

	var body struct {
		ResumeData any `json:"resumeData"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Printf("resume.validate invalid_json remote=%s err=%v", r.RemoteAddr, err)
		writeError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	result := validateResumeData(body.ResumeData)
	if !result.Valid {
		log.Printf("resume.validate failed remote=%s errors=%d", r.RemoteAddr, len(result.Errors))
		writeError(w, http.StatusBadRequest, "Resume schema validation failed", result.Errors...)
		return
	}
	log.Printf("resume.validate success remote=%s", r.RemoteAddr)
	writeJSON(w, http.StatusOK, result)
}

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

	installations, _ := getGitHubInstallationsFromToken(token)
	login, _ := me["login"].(string)
	chosenInstallation := pickInstallationForUser(installations, login)

	if chosenInstallation == nil {
		installHint := "Install the GitHub App for your account/repository and try again."
		if appInstallURL != "" {
			installHint = fmt.Sprintf("Install the app first: %s", appInstallURL)
		}
		writeError(w, http.StatusBadRequest, "GitHub App is not installed for this account.", installHint)
		return
	}

	var installationID int64
	if id, ok := chosenInstallation["id"].(float64); ok {
		installationID = int64(id)
	}

	result, err := createRepositoryAndDeployTheme(token, login, installationID, params)
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
