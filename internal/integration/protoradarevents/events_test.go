package protoradarevents

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

func TestNewModuleCreated(t *testing.T) {
	occurredAt := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	moduleName, err := domain.NewModuleName("billing_api")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}

	record, err := NewModuleCreated(domain.Module{
		ID:            domain.NewModuleID("module-1"),
		Name:          moduleName,
		RepositoryURL: "https://gitlab.example.com/platform/billing-api",
	}, occurredAt)
	if err != nil {
		t.Fatalf("event: %v", err)
	}

	if record.EventType != EventTypeModuleCreated {
		t.Fatalf("event type = %q, want %q", record.EventType, EventTypeModuleCreated)
	}
	if record.DedupKey != "module:module-1:created" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}

	var payload ModuleCreatedPayload
	if err := json.Unmarshal(record.Payload, &payload); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	if payload.ModuleID != "module-1" {
		t.Fatalf("module_id = %q", payload.ModuleID)
	}
	if payload.ModuleName != "billing_api" {
		t.Fatalf("module_name = %q", payload.ModuleName)
	}
	if payload.RepositoryURL != "https://gitlab.example.com/platform/billing-api" {
		t.Fatalf("repository_url = %q", payload.RepositoryURL)
	}
	if !payload.OccurredAt.Equal(occurredAt) {
		t.Fatalf("occurred_at = %s, want %s", payload.OccurredAt, occurredAt)
	}
	assertNoRawTokenOrInfrastructureDetails(t, string(record.Payload))
	assertNoRawTokenOrInfrastructureDetails(t, record.EventType)
	assertNoRawTokenOrInfrastructureDetails(t, record.DedupKey)
}

func TestNewModuleVersionPublished(t *testing.T) {
	occurredAt := time.Date(2026, 6, 4, 10, 30, 0, 0, time.UTC)
	moduleName, err := domain.NewModuleName("billing-api")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	versionValue, err := domain.NewVersion("v1.2.3")
	if err != nil {
		t.Fatalf("version: %v", err)
	}

	record, err := NewModuleVersionPublished(
		domain.Module{
			ID:   domain.NewModuleID("module-1"),
			Name: moduleName,
		},
		domain.ModuleVersion{
			ID:       domain.NewModuleVersionID("module-version-1"),
			ModuleID: domain.NewModuleID("module-1"),
			Version:  versionValue,
			Digest:   "sha256:descriptor-digest",
		},
		domain.Artifact{
			ID:              domain.NewArtifactID("artifact-1"),
			ModuleVersionID: domain.NewModuleVersionID("module-version-1"),
			ChecksumSHA256:  "artifact-checksum",
			SizeBytes:       128,
		},
		occurredAt,
	)
	if err != nil {
		t.Fatalf("event: %v", err)
	}

	if record.EventType != EventTypeModuleVersionPublished {
		t.Fatalf("event type = %q, want %q", record.EventType, EventTypeModuleVersionPublished)
	}
	if record.DedupKey != "module:module-1:version:v1.2.3:published" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}

	var payload ModuleVersionPublishedPayload
	if err := json.Unmarshal(record.Payload, &payload); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	if payload.ModuleID != "module-1" {
		t.Fatalf("module_id = %q", payload.ModuleID)
	}
	if payload.ModuleName != "billing-api" {
		t.Fatalf("module_name = %q", payload.ModuleName)
	}
	if payload.ModuleVersionID != "module-version-1" {
		t.Fatalf("module_version_id = %q", payload.ModuleVersionID)
	}
	if payload.Version != "v1.2.3" {
		t.Fatalf("version = %q", payload.Version)
	}
	if payload.Digest != "sha256:descriptor-digest" {
		t.Fatalf("digest = %q", payload.Digest)
	}
	if payload.ArtifactChecksumSHA256 != "artifact-checksum" {
		t.Fatalf("artifact_checksum_sha256 = %q", payload.ArtifactChecksumSHA256)
	}
	if payload.ArtifactSizeBytes != 128 {
		t.Fatalf("artifact_size_bytes = %d", payload.ArtifactSizeBytes)
	}
	if !payload.OccurredAt.Equal(occurredAt) {
		t.Fatalf("occurred_at = %s, want %s", payload.OccurredAt, occurredAt)
	}
	assertNoRawTokenOrInfrastructureDetails(t, string(record.Payload))
	assertNoRawTokenOrInfrastructureDetails(t, record.EventType)
	assertNoRawTokenOrInfrastructureDetails(t, record.DedupKey)
}

func assertNoRawTokenOrInfrastructureDetails(t *testing.T, value string) {
	t.Helper()

	for _, forbidden := range []string{
		"raw_token",
		"api_token_value",
		"secret",
		"bearer",
		"sarama",
		"kafka",
		"postgres",
		"s3",
		"minio",
	} {
		if strings.Contains(strings.ToLower(value), forbidden) {
			t.Fatalf("value contains forbidden detail %q: %s", forbidden, value)
		}
	}
}
