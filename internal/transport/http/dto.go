package httptransport

import (
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type errorResponse struct {
	Error string `json:"error"`
}

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

type moduleVersionDTO struct {
	ID          string     `json:"id"`
	ModuleID    string     `json:"module_id"`
	Version     string     `json:"version"`
	Digest      string     `json:"digest"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

type listModuleVersionsResponse struct {
	Versions []moduleVersionDTO `json:"versions"`
}

type artifactDTO struct {
	ID              string    `json:"id"`
	ModuleVersionID string    `json:"module_version_id"`
	StorageKey      string    `json:"storage_key"`
	ChecksumSHA256  string    `json:"checksum_sha256"`
	SizeBytes       int64     `json:"size_bytes"`
	CreatedAt       time.Time `json:"created_at"`
}

type publishModuleVersionResponse struct {
	Version  moduleVersionDTO `json:"version"`
	Artifact artifactDTO      `json:"artifact"`
}

type createAPITokenRequest struct {
	Name      string `json:"name"`
	ExpiresAt string `json:"expires_at"`
}

type createAPITokenResponse struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Token     string     `json:"token"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
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

func moduleVersionResponse(version domain.ModuleVersion) moduleVersionDTO {
	return moduleVersionDTO{
		ID:          version.ID.String(),
		ModuleID:    version.ModuleID.String(),
		Version:     version.Version.String(),
		Digest:      version.Digest,
		Status:      version.Status.String(),
		CreatedAt:   version.CreatedAt,
		PublishedAt: version.PublishedAt,
	}
}

func artifactResponse(artifact domain.Artifact) artifactDTO {
	return artifactDTO{
		ID:              artifact.ID.String(),
		ModuleVersionID: artifact.ModuleVersionID.String(),
		StorageKey:      artifact.StorageKey,
		ChecksumSHA256:  artifact.ChecksumSHA256,
		SizeBytes:       artifact.SizeBytes,
		CreatedAt:       artifact.CreatedAt,
	}
}
