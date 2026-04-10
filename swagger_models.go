package main

// GitHubUser represents the GitHub user info
type GitHubUser struct {
	Login     string `json:"login" example:"octocat"`
	Name      string `json:"name" example:"The Octocat"`
	AvatarUrl string `json:"avatarUrl" example:"https://github.com/images/octocat.png"`
}

// GitHubAppInfo represents the GitHub app installation info
type GitHubAppInfo struct {
	Installed      bool   `json:"installed" example:"true"`
	InstallationId int64  `json:"installationId" example:"12345"`
	InstallUrl     string `json:"installUrl" example:"https://github.com/apps/myapp/installations/new"`
}

// MeResponse represents the response for /api/github/me
type MeResponse struct {
	User      GitHubUser    `json:"user"`
	GitHubApp GitHubAppInfo `json:"githubApp"`
}

// ValidateRequest represents the validation request body
type ValidateRequest struct {
	ResumeData any `json:"resumeData"`
}

// ValidationResult represents the validation result
type ValidationResult struct {
	Valid  bool     `json:"valid" example:"true"`
	Errors []string `json:"errors"`
}
