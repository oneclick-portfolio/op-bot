package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type authSession struct {
	Token     string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type authSessionManager struct {
	mu       sync.RWMutex
	sessions map[string]authSession
	ttl      time.Duration
}

func newAuthSessionManager(ttl time.Duration) *authSessionManager {
	if ttl <= 0 {
		ttl = 4 * time.Hour
	}
	return &authSessionManager{
		sessions: make(map[string]authSession),
		ttl:      ttl,
	}
}

func (m *authSessionManager) create(token string) (string, error) {
	if token == "" {
		return "", fmt.Errorf("empty token")
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	key := base64.RawURLEncoding.EncodeToString(raw)
	hash := hashSessionKey(key)

	m.mu.Lock()
	m.sessions[hash] = authSession{
		Token:     token,
		ExpiresAt: time.Now().Add(m.ttl),
	}
	m.mu.Unlock()

	return key, nil
}

func (m *authSessionManager) resolve(key string) (string, bool) {
	if key == "" {
		return "", false
	}
	hash := hashSessionKey(key)

	m.mu.RLock()
	session, ok := m.sessions[hash]
	m.mu.RUnlock()
	if !ok {
		return "", false
	}

	if session.RevokedAt != nil || time.Now().After(session.ExpiresAt) {
		m.revoke(key)
		return "", false
	}

	return session.Token, true
}

func (m *authSessionManager) revoke(key string) {
	if key == "" {
		return
	}
	hash := hashSessionKey(key)

	m.mu.Lock()
	session, ok := m.sessions[hash]
	if ok {
		now := time.Now()
		session.RevokedAt = &now
		m.sessions[hash] = session
	}
	m.mu.Unlock()
}

func hashSessionKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func bearerSessionKey(r *http.Request) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return ""
	}
	return strings.TrimSpace(authHeader[len("Bearer "):])
}

func authTokenFromRequest(r *http.Request) (string, bool) {
	key := bearerSessionKey(r)
	if key == "" || sessionManager == nil {
		return "", false
	}
	return sessionManager.resolve(key)
}
