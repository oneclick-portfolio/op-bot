package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type fileEntry struct {
	Path     string
	Content  []byte
	Encoding string
}

func readTextFile(relativePath string) (string, error) {
	data, err := os.ReadFile(filepath.Join(frontendDir, relativePath))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func readBinaryFile(relativePath string) ([]byte, error) {
	return os.ReadFile(filepath.Join(frontendDir, relativePath))
}

func buildWorkflowYaml() string {
	return `name: Deploy GitHub Pages

on:
  push:
    branches: [main]
  workflow_dispatch:

permissions:
  contents: read
  pages: write
  id-token: write

concurrency:
  group: pages
  cancel-in-progress: true

jobs:
  deploy:
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Setup Pages
        uses: actions/configure-pages@v5

      - name: Upload artifact
        uses: actions/upload-pages-artifact@v3
        with:
          path: .

      - name: Deploy to GitHub Pages
        id: deployment
        uses: actions/deploy-pages@v4
        with:
          enablement: true
`
}

func buildRepositoryReadme(themeName, pagesURL string) string {
	return fmt.Sprintf(`# Portfolio (%s)

This repository was generated automatically from the portfolio theme deployer.

## Live Site

%s

## Local Preview

`+"`python3 -m http.server 8080`"+`
`, themeName, pagesURL)
}

func transformThemeIndex(indexHTML string) string {
	indexHTML = strings.ReplaceAll(indexHTML, "../../config.js", "./config.js")
	indexHTML = strings.ReplaceAll(indexHTML, "../../src/rxresume.js", "./src/rxresume.js")
	return indexHTML
}

func buildThemeBundle(theme string, resumeData any) ([]fileEntry, error) {
	selection := themeFiles[theme]

	indexHTML, err := readTextFile(selection["index"])
	if err != nil {
		return nil, err
	}
	appJS, err := readTextFile(selection["app"])
	if err != nil {
		return nil, err
	}
	styleCSS, err := readTextFile(selection["style"])
	if err != nil {
		return nil, err
	}
	configJS, err := readTextFile("config.js")
	if err != nil {
		return nil, err
	}
	rxresumeJS, err := readTextFile("src/rxresume.js")
	if err != nil {
		return nil, err
	}

	var resumeJSON string
	if resumeData != nil {
		data, err := json.MarshalIndent(resumeData, "", "  ")
		if err != nil {
			return nil, err
		}
		resumeJSON = string(data) + "\n"
	} else {
		resumeJSON, err = readTextFile("resume/Reactive Resume.json")
		if err != nil {
			return nil, err
		}
	}

	faviconSVG, err := readTextFile("favicon.svg")
	if err != nil {
		return nil, err
	}
	faviconICO, err := readBinaryFile("favicon.ico")
	if err != nil {
		return nil, err
	}

	styleFileName := path.Base(selection["style"])
	pagesURLPlaceholder := "https://<username>.github.io/<repository>/"

	return []fileEntry{
		{Path: "index.html", Content: []byte(transformThemeIndex(indexHTML)), Encoding: "utf-8"},
		{Path: "app.js", Content: []byte(appJS), Encoding: "utf-8"},
		{Path: styleFileName, Content: []byte(styleCSS), Encoding: "utf-8"},
		{Path: "config.js", Content: []byte(configJS), Encoding: "utf-8"},
		{Path: "src/rxresume.js", Content: []byte(rxresumeJS), Encoding: "utf-8"},
		{Path: "resume/Reactive Resume.json", Content: []byte(resumeJSON), Encoding: "utf-8"},
		{Path: "favicon.svg", Content: []byte(faviconSVG), Encoding: "utf-8"},
		{Path: "favicon.ico", Content: faviconICO, Encoding: "binary"},
		{Path: ".github/workflows/deploy-pages.yml", Content: []byte(buildWorkflowYaml()), Encoding: "utf-8"},
		{Path: "README.md", Content: []byte(buildRepositoryReadme(getThemeLabel(theme), pagesURLPlaceholder)), Encoding: "utf-8"},
	}, nil
}
