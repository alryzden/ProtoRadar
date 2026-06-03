package protoradarevents

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/outbox"
)

const EventTypeModuleVersionPublished = "protoradar.module_version.published"

type ModuleVersionPublished struct {
	Module           domain.Module
	Version          domain.ModuleVersion
	SourceArtifact   domain.Artifact
	BufImageArtifact domain.Artifact
	BufConfig        domain.BufConfigInfo
	LintResult       domain.BufLintResult
	MetadataSummary  domain.DescriptorMetadataSummary
	OccurredAt       time.Time
}

type ModuleVersionPublishedPayload struct {
	ModuleID                     string                           `json:"module_id"`
	ModuleName                   string                           `json:"module_name"`
	ModuleVersionID              string                           `json:"module_version_id"`
	Version                      string                           `json:"version"`
	Digest                       string                           `json:"digest"`
	SourceArtifactChecksumSHA256 string                           `json:"source_artifact_checksum_sha256"`
	SourceArtifactSizeBytes      int64                            `json:"source_artifact_size_bytes"`
	BufImageChecksumSHA256       string                           `json:"buf_image_checksum_sha256"`
	BufImageSizeBytes            int64                            `json:"buf_image_size_bytes"`
	BufYAMLPresent               bool                             `json:"buf_yaml_present"`
	BufLockPresent               bool                             `json:"buf_lock_present"`
	LintStatus                   string                           `json:"lint_status"`
	DescriptorSummary            DescriptorMetadataSummaryPayload `json:"descriptor_summary"`
	OccurredAt                   time.Time                        `json:"occurred_at"`
}

type DescriptorMetadataSummaryPayload struct {
	FileCount      int `json:"file_count"`
	ImportCount    int `json:"import_count"`
	ServiceCount   int `json:"service_count"`
	MethodCount    int `json:"method_count"`
	MessageCount   int `json:"message_count"`
	FieldCount     int `json:"field_count"`
	EnumCount      int `json:"enum_count"`
	EnumValueCount int `json:"enum_value_count"`
}

func NewModuleVersionPublished(event ModuleVersionPublished) (outbox.Record, error) {
	lintStatus := event.LintResult.Status.String()
	if lintStatus == "" {
		lintStatus = domain.BufLintStatusNotRun.String()
	}

	payload := ModuleVersionPublishedPayload{
		ModuleID:                     event.Module.ID.String(),
		ModuleName:                   event.Module.Name.String(),
		ModuleVersionID:              event.Version.ID.String(),
		Version:                      event.Version.Version.String(),
		Digest:                       event.Version.Digest,
		SourceArtifactChecksumSHA256: event.SourceArtifact.ChecksumSHA256,
		SourceArtifactSizeBytes:      event.SourceArtifact.SizeBytes,
		BufImageChecksumSHA256:       event.BufImageArtifact.ChecksumSHA256,
		BufImageSizeBytes:            event.BufImageArtifact.SizeBytes,
		BufYAMLPresent:               event.BufConfig.BufYAMLPresent,
		BufLockPresent:               event.BufConfig.BufLockPresent,
		LintStatus:                   lintStatus,
		DescriptorSummary:            descriptorMetadataSummaryPayload(event.MetadataSummary),
		OccurredAt:                   event.OccurredAt,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return outbox.Record{}, err
	}

	moduleID := event.Module.ID.String()
	return outbox.Record{
		AggregateType: aggregateTypeModule,
		AggregateID:   moduleID,
		EventType:     EventTypeModuleVersionPublished,
		DedupKey:      fmt.Sprintf("module:%s:version:%s:published", moduleID, event.Version.Version.String()),
		Payload:       body,
		OccurredAt:    event.OccurredAt,
	}, nil
}

func descriptorMetadataSummaryPayload(summary domain.DescriptorMetadataSummary) DescriptorMetadataSummaryPayload {
	return DescriptorMetadataSummaryPayload{
		FileCount:      summary.FileCount,
		ImportCount:    summary.ImportCount,
		ServiceCount:   summary.ServiceCount,
		MethodCount:    summary.MethodCount,
		MessageCount:   summary.MessageCount,
		FieldCount:     summary.FieldCount,
		EnumCount:      summary.EnumCount,
		EnumValueCount: summary.EnumValueCount,
	}
}
