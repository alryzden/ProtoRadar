package httptransport

import (
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type createModuleRequest struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	RepositoryURL string `json:"repository_url"`
}

type moduleDTO struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	RepositoryURL string    `json:"repository_url"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type listModulesResponse struct {
	Modules []moduleDTO `json:"modules"`
}

type linkModuleGitLabProjectRequest struct {
	GitLabBaseURL     string `json:"gitlab_base_url"`
	GitLabProjectID   int64  `json:"gitlab_project_id"`
	GitLabProjectPath string `json:"gitlab_project_path"`
}

type moduleGitLabProjectDTO struct {
	Module            string    `json:"module"`
	GitLabBaseURL     string    `json:"gitlab_base_url"`
	GitLabProjectID   int64     `json:"gitlab_project_id"`
	GitLabProjectPath string    `json:"gitlab_project_path"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type moduleVersionDTO struct {
	ID                string     `json:"id"`
	ModuleID          string     `json:"module_id"`
	Version           string     `json:"version"`
	Digest            string     `json:"digest"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"created_at"`
	PublishedAt       *time.Time `json:"published_at,omitempty"`
	DeprecatedAt      *time.Time `json:"deprecated_at,omitempty"`
	DeprecatedBy      string     `json:"deprecated_by"`
	DeprecationReason string     `json:"deprecation_reason"`
}

type listModuleVersionsResponse struct {
	Versions []moduleVersionDTO `json:"versions"`
}
type deprecateModuleVersionRequest struct {
	Reason string `json:"reason"`
}

func moduleResponse(module domain.Module) moduleDTO {
	return moduleDTO{
		ID:            module.ID.String(),
		Name:          module.Name.String(),
		Description:   module.Description,
		RepositoryURL: module.RepositoryURL,
		CreatedAt:     module.CreatedAt,
		UpdatedAt:     module.UpdatedAt,
	}
}

func moduleGitLabProjectResponse(mapping domain.ModuleGitLabProject) moduleGitLabProjectDTO {
	return moduleGitLabProjectDTO{
		Module:            mapping.ModuleName.String(),
		GitLabBaseURL:     mapping.GitLabBaseURL,
		GitLabProjectID:   mapping.GitLabProjectID,
		GitLabProjectPath: mapping.GitLabProjectPath,
		CreatedAt:         mapping.CreatedAt,
		UpdatedAt:         mapping.UpdatedAt,
	}
}

func moduleVersionResponse(version domain.ModuleVersion) moduleVersionDTO {
	return moduleVersionDTO{
		ID:                version.ID.String(),
		ModuleID:          version.ModuleID.String(),
		Version:           version.Version.String(),
		Digest:            version.Digest,
		Status:            version.Status.String(),
		CreatedAt:         version.CreatedAt,
		PublishedAt:       version.PublishedAt,
		DeprecatedAt:      version.DeprecatedAt,
		DeprecatedBy:      version.DeprecatedBy,
		DeprecationReason: version.DeprecationReason,
	}
}
