package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"op-bot/internal/models"
	"time"
)

// Service provides GitHub App authentication operations.
type Service struct {
	AppID      string
	PrivateKey *rsa.PrivateKey
}

// NewService creates a new auth service.
func NewService(appID string, privateKey *rsa.PrivateKey) *Service {
	return &Service{
		AppID:      appID,
		PrivateKey: privateKey,
	}
}

// GenerateAppJWT generates a GitHub App JWT token.
func (s *Service) GenerateAppJWT() (string, error) {
	if s.PrivateKey == nil {
		return "", fmt.Errorf("APP_PRIVATE_KEY not configured")
	}
	if s.AppID == "" {
		return "", fmt.Errorf("APP_ID not configured")
	}

	now := time.Now().Unix()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"iat":%d,"exp":%d,"iss":"%s"}`, now-60, now+600, s.AppID)))

	unsigned := header + "." + payload
	h := crypto.SHA256.New()
	h.Write([]byte(unsigned))
	digest := h.Sum(nil)

	sig, err := rsa.SignPKCS1v15(rand.Reader, s.PrivateKey, crypto.SHA256, digest)
	if err != nil {
		return "", fmt.Errorf("unable to sign app JWT: %w", err)
	}

	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// GetInstallationToken exchanges a GitHub App JWT for an installation token.
func (s *Service) GetInstallationToken(installationID int64) (*models.InstallationToken, error) {
	jwt, err := s.GenerateAppJWT()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to exchange installation token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read installation token response: %w", err)
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unable to parse installation token response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		msg := "installation token exchange failed"
		if m, ok := result["message"].(string); ok {
			msg = m
		}
		return nil, fmt.Errorf("%s: status %d", msg, resp.StatusCode)
	}

	token, ok := result["token"].(string)
	if !ok || token == "" {
		return nil, fmt.Errorf("empty installation token in response")
	}

	permissions := map[string]string{}
	if rawPermissions, ok := result["permissions"].(map[string]any); ok {
		for key, value := range rawPermissions {
			if text, ok := value.(string); ok {
				permissions[key] = text
			}
		}
	}

	selection, _ := result["repository_selection"].(string)
	singleFileName, _ := result["single_file_name"].(string)
	expiresAt, _ := result["expires_at"].(string)

	return &models.InstallationToken{
		Token:               token,
		RepositorySelection: selection,
		SingleFileName:      singleFileName,
		Permissions:         permissions,
		ExpiresAt:           expiresAt,
	}, nil
}
