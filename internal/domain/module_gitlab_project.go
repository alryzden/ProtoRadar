package domain

import (
	"net/url"
	"strings"
	"time"
)

type ModuleGitLabProject struct {
	ID                ModuleGitLabProjectID
	ModuleID          ModuleID
	ModuleName        ModuleName
	GitLabBaseURL     string
	GitLabProjectID   int64
	GitLabProjectPath string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (mapping ModuleGitLabProject) Validate() error {
	_, err := mapping.Normalized()
	return err
}

func (mapping ModuleGitLabProject) Normalized() (ModuleGitLabProject, error) {
	moduleName, err := NewModuleName(mapping.ModuleName.String())
	if err != nil {
		return ModuleGitLabProject{}, err
	}

	baseURL := strings.TrimSpace(mapping.GitLabBaseURL)
	parsed, err := url.Parse(baseURL)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return ModuleGitLabProject{}, ErrInvalidGitLabBaseURL
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ModuleGitLabProject{}, ErrInvalidGitLabBaseURL
	}

	if mapping.GitLabProjectID <= 0 {
		return ModuleGitLabProject{}, ErrInvalidGitLabProjectID
	}

	projectPath := strings.TrimSpace(mapping.GitLabProjectPath)
	if projectPath == "" || strings.ContainsAny(projectPath, " \t\r\n") {
		return ModuleGitLabProject{}, ErrInvalidGitLabProjectPath
	}

	mapping.ModuleName = moduleName
	mapping.GitLabBaseURL = strings.TrimRight(baseURL, "/")
	mapping.GitLabProjectPath = projectPath
	return mapping, nil
}
