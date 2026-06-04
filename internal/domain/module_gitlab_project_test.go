package domain

import (
	"errors"
	"testing"
)

func TestModuleGitLabProjectValidationAcceptsValidMapping(t *testing.T) {
	moduleName, err := NewModuleName("user-api")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	mapping := ModuleGitLabProject{
		ID:                NewModuleGitLabProjectID("mapping-1"),
		ModuleID:          NewModuleID("module-1"),
		ModuleName:        moduleName,
		GitLabBaseURL:     " https://gitlab.example.com/ ",
		GitLabProjectID:   123,
		GitLabProjectPath: "platform/user-api",
	}

	normalized, err := mapping.Normalized()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if normalized.GitLabBaseURL != "https://gitlab.example.com" {
		t.Fatalf("gitlab base url = %q", normalized.GitLabBaseURL)
	}
	if normalized.GitLabProjectPath != "platform/user-api" {
		t.Fatalf("gitlab project path = %q", normalized.GitLabProjectPath)
	}
}

func TestModuleGitLabProjectValidationRejectsInvalidGitLabBaseURL(t *testing.T) {
	mapping := validModuleGitLabProject(t)
	mapping.GitLabBaseURL = "://not-a-url"

	err := mapping.Validate()
	if !errors.Is(err, ErrInvalidGitLabBaseURL) {
		t.Fatalf("error = %v, want ErrInvalidGitLabBaseURL", err)
	}
}

func TestModuleGitLabProjectValidationRejectsNonHTTPGitLabBaseURL(t *testing.T) {
	mapping := validModuleGitLabProject(t)
	mapping.GitLabBaseURL = "ssh://gitlab.example.com"

	err := mapping.Validate()
	if !errors.Is(err, ErrInvalidGitLabBaseURL) {
		t.Fatalf("error = %v, want ErrInvalidGitLabBaseURL", err)
	}
}

func TestModuleGitLabProjectValidationRejectsZeroOrNegativeProjectID(t *testing.T) {
	for _, projectID := range []int64{0, -1} {
		mapping := validModuleGitLabProject(t)
		mapping.GitLabProjectID = projectID

		err := mapping.Validate()
		if !errors.Is(err, ErrInvalidGitLabProjectID) {
			t.Fatalf("project id %d error = %v, want ErrInvalidGitLabProjectID", projectID, err)
		}
	}
}

func TestModuleGitLabProjectValidationRejectsEmptyProjectPath(t *testing.T) {
	for _, projectPath := range []string{"", "   ", "platform/user api"} {
		mapping := validModuleGitLabProject(t)
		mapping.GitLabProjectPath = projectPath

		err := mapping.Validate()
		if !errors.Is(err, ErrInvalidGitLabProjectPath) {
			t.Fatalf("project path %q error = %v, want ErrInvalidGitLabProjectPath", projectPath, err)
		}
	}
}

func validModuleGitLabProject(t *testing.T) ModuleGitLabProject {
	t.Helper()
	moduleName, err := NewModuleName("user-api")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	return ModuleGitLabProject{
		ID:                NewModuleGitLabProjectID("mapping-1"),
		ModuleID:          NewModuleID("module-1"),
		ModuleName:        moduleName,
		GitLabBaseURL:     "https://gitlab.example.com",
		GitLabProjectID:   123,
		GitLabProjectPath: "platform/user-api",
	}
}
