package protoradarevents

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/outbox"
)

const EventTypeModuleVersionPublished = "protoradar.module_version.published"

type ModuleVersionPublishedPayload struct {
	ModuleID               string    `json:"module_id"`
	ModuleName             string    `json:"module_name"`
	ModuleVersionID        string    `json:"module_version_id"`
	Version                string    `json:"version"`
	Digest                 string    `json:"digest"`
	ArtifactChecksumSHA256 string    `json:"artifact_checksum_sha256"`
	ArtifactSizeBytes      int64     `json:"artifact_size_bytes"`
	OccurredAt             time.Time `json:"occurred_at"`
}

func NewModuleVersionPublished(module domain.Module, version domain.ModuleVersion, artifact domain.Artifact, occurredAt time.Time) (outbox.Record, error) {
	payload := ModuleVersionPublishedPayload{
		ModuleID:               module.ID.String(),
		ModuleName:             module.Name.String(),
		ModuleVersionID:        version.ID.String(),
		Version:                version.Version.String(),
		Digest:                 version.Digest,
		ArtifactChecksumSHA256: artifact.ChecksumSHA256,
		ArtifactSizeBytes:      artifact.SizeBytes,
		OccurredAt:             occurredAt,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return outbox.Record{}, err
	}

	moduleID := module.ID.String()
	return outbox.Record{
		AggregateType: aggregateTypeModule,
		AggregateID:   moduleID,
		EventType:     EventTypeModuleVersionPublished,
		DedupKey:      fmt.Sprintf("module:%s:version:%s:published", moduleID, version.Version.String()),
		Payload:       body,
		OccurredAt:    occurredAt,
	}, nil
}
