package models

type DeployParams struct {
	Theme             string `json:"theme"`
	RepositoryName    string `json:"repositoryName"`
	RepositoryOwner   string `json:"repositoryOwner,omitempty"`
	PrivateRepo       bool   `json:"privateRepo"`
	ResumeData        any    `json:"resumeData"`
	ThemeRepoLink     string `json:"themeRepoLink"`
	Description       string `json:"description,omitempty"`
	HomepageURL       string `json:"homepageUrl,omitempty"`
	UseGitHubPagesURL bool   `json:"useGitHubPagesUrl,omitempty"`
}

type ParsedThemeRepo struct {
	Repo   string
	Ref    string
	SubDir string
}

type DeployResult struct {
	RepositoryURL  string `json:"repositoryUrl"`
	PagesURL       string `json:"pagesUrl"`
	RepoFullName   string `json:"repoFullName"`
	ReusedExisting bool   `json:"reusedExistingRepo"`
	InstallationID any    `json:"installationId,omitempty"`
}
