package main

import (
	"op-bot/internal/auth"
	"op-bot/internal/models"
)

// Create a module-level auth service for backwards compatibility.
var authService *auth.Service

// initAuthService initializes the auth service (called during LoadAppContext).
func initAuthService() {
	authService = auth.NewService(appID, appPrivateKey)
}

// generateAppJWT is a wrapper for backwards compatibility.
func generateAppJWT() (string, error) {
	if authService == nil {
		initAuthService()
	}
	return authService.GenerateAppJWT()
}

// getInstallationToken is a wrapper for backwards compatibility.
func getInstallationToken(installationID int64) (*models.InstallationToken, error) {
	if authService == nil {
		initAuthService()
	}
	return authService.GetInstallationToken(installationID)
}
