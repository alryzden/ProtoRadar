package httptransport

import (
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
)

type artifactSummaryDTO struct {
	Kind           string `json:"kind"`
	SizeBytes      int64  `json:"size_bytes"`
	ChecksumSHA256 string `json:"checksum_sha256"`
}

type bufInfoDTO struct {
	ConfigPresent         bool     `json:"config_present"`
	LockPresent           bool     `json:"lock_present"`
	BufYAMLDigest         string   `json:"buf_yaml_digest,omitempty"`
	BufLockDigest         string   `json:"buf_lock_digest,omitempty"`
	ModulePaths           []string `json:"module_paths,omitempty"`
	Deps                  []string `json:"deps,omitempty"`
	LintEnabled           bool     `json:"lint_enabled"`
	LintStatus            string   `json:"lint_status"`
	LintReport            string   `json:"lint_report,omitempty"`
	BreakingConfigPresent bool     `json:"breaking_config_present"`
}
type publishModuleVersionResponse struct {
	Module           string             `json:"module"`
	Version          string             `json:"version"`
	Status           string             `json:"status"`
	SourceArtifact   artifactSummaryDTO `json:"source_artifact"`
	BufImageArtifact artifactSummaryDTO `json:"buf_image_artifact"`
	Buf              bufInfoDTO         `json:"buf"`
	MetadataSummary  metadataSummaryDTO `json:"metadata_summary"`
	CreatedAt        time.Time          `json:"created_at"`
}
type moduleVersionDetailsDTO struct {
	ID                string               `json:"id"`
	ModuleID          string               `json:"module_id"`
	Module            string               `json:"module"`
	Version           string               `json:"version"`
	Digest            string               `json:"digest"`
	Status            string               `json:"status"`
	Artifacts         []artifactSummaryDTO `json:"artifacts"`
	Buf               bufInfoDTO           `json:"buf"`
	MetadataSummary   metadataSummaryDTO   `json:"metadata_summary"`
	CreatedAt         time.Time            `json:"created_at"`
	PublishedAt       *time.Time           `json:"published_at,omitempty"`
	DeprecatedAt      *time.Time           `json:"deprecated_at,omitempty"`
	DeprecatedBy      string               `json:"deprecated_by"`
	DeprecationReason string               `json:"deprecation_reason"`
}

func artifactSummaryResponse(artifact domain.Artifact) artifactSummaryDTO {
	return artifactSummaryDTO{
		Kind:           artifact.Kind.String(),
		SizeBytes:      artifact.SizeBytes,
		ChecksumSHA256: artifact.ChecksumSHA256,
	}
}

func bufInfoResponse(config domain.BufConfigInfo, lint domain.BufLintResult) bufInfoDTO {
	lintStatus := lint.Status.String()
	if lintStatus == "" {
		lintStatus = domain.BufLintStatusNotRun.String()
	}
	return bufInfoDTO{
		ConfigPresent:         config.BufYAMLPresent,
		LockPresent:           config.BufLockPresent,
		BufYAMLDigest:         config.BufYAMLDigest,
		BufLockDigest:         config.BufLockDigest,
		ModulePaths:           config.ModulePaths,
		Deps:                  config.Deps,
		LintEnabled:           config.LintEnabled,
		LintStatus:            lintStatus,
		LintReport:            lint.Report,
		BreakingConfigPresent: config.BreakingConfigPresent,
	}
}
func moduleVersionDetailsResponse(moduleName string, details registry.ModuleVersionDetailsResponse) moduleVersionDetailsDTO {
	artifacts := make([]artifactSummaryDTO, 0, len(details.Artifacts))
	for _, artifact := range details.Artifacts {
		artifacts = append(artifacts, artifactSummaryResponse(artifact))
	}
	return moduleVersionDetailsDTO{
		ID:                details.Version.ID.String(),
		ModuleID:          details.Version.ModuleID.String(),
		Module:            moduleName,
		Version:           details.Version.Version.String(),
		Digest:            details.Version.Digest,
		Status:            details.Version.Status.String(),
		Artifacts:         artifacts,
		Buf:               bufInfoResponse(details.BufConfig, details.LintResult),
		MetadataSummary:   metadataSummaryResponse(details.MetadataSummary),
		CreatedAt:         details.Version.CreatedAt,
		PublishedAt:       details.Version.PublishedAt,
		DeprecatedAt:      details.Version.DeprecatedAt,
		DeprecatedBy:      details.Version.DeprecatedBy,
		DeprecationReason: details.Version.DeprecationReason,
	}
}
