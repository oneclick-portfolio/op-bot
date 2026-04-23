package models

type InstallationToken struct {
	Token               string
	RepositorySelection string
	SingleFileName      string
	Permissions         map[string]string
	ExpiresAt           string
}
