package services

func BuildGitHubMeResponse(user map[string]any, chosenInstallation map[string]any, appInstallURL string) map[string]any {
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

	return map[string]any{
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
	}
}

func BuildGitHubReposResponse(installationID any, repos []map[string]any) map[string]any {
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

	return map[string]any{
		"installationId": installationID,
		"repositories":   result,
	}
}
