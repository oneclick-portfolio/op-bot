package models

type APIErrorPayload struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

type APIErrorResponse struct {
	Error APIErrorPayload `json:"error"`
}

// GitHubUser represents the GitHub user info.
type GitHubUser struct {
	Login     string `json:"login" example:"octocat"`
	Name      string `json:"name" example:"The Octocat"`
	AvatarURL string `json:"avatarUrl" example:"https://github.com/images/octocat.png"`
}

// GitHubAppInfo represents the GitHub app installation info.
type GitHubAppInfo struct {
	Installed      bool   `json:"installed" example:"true"`
	InstallationID int64  `json:"installationId" example:"12345"`
	InstallURL     string `json:"installUrl" example:"https://github.com/apps/myapp"`
}

// MeResponse represents the response for /api/github/me.
type MeResponse struct {
	User      GitHubUser    `json:"user"`
	GitHubApp GitHubAppInfo `json:"githubApp"`
}

// ValidateRequest represents the validation request body.
type ValidateRequest struct {
	ResumeData any `json:"resumeData"`
}

// ValidationResult represents the validation result.
type ValidationResult struct {
	Valid  bool     `json:"valid" example:"true"`
	Errors []string `json:"errors"`
}
