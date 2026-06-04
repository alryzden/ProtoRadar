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

	record, err := NewModuleVersionPublished(ModuleVersionPublished{
		Module: domain.Module{
			ID:   domain.NewModuleID("module-1"),
			Name: moduleName,
		},
		Version: domain.ModuleVersion{
			ID:       domain.NewModuleVersionID("module-version-1"),
			ModuleID: domain.NewModuleID("module-1"),
			Version:  versionValue,
			Digest:   "sha256:descriptor-digest",
		},
		SourceArtifact: domain.Artifact{
			ID:              domain.NewArtifactID("source-artifact-1"),
			ModuleVersionID: domain.NewModuleVersionID("module-version-1"),
			Kind:            domain.ArtifactKindSourceArchive,
			ChecksumSHA256:  "source-checksum",
			SizeBytes:       128,
		},
		BufImageArtifact: domain.Artifact{
			ID:              domain.NewArtifactID("buf-image-artifact-1"),
			ModuleVersionID: domain.NewModuleVersionID("module-version-1"),
			Kind:            domain.ArtifactKindBufImage,
			ChecksumSHA256:  "buf-image-checksum",
			SizeBytes:       256,
		},
		BufConfig: domain.BufConfigInfo{
			BufYAMLPresent: true,
			BufLockPresent: true,
		},
		LintResult: domain.BufLintResult{
			Status: domain.BufLintStatusWarning,
			Report: "lint warning text is intentionally not emitted",
		},
		MetadataSummary: domain.DescriptorMetadataSummary{
			FileCount:      2,
			ImportCount:    3,
			ServiceCount:   4,
			MethodCount:    5,
			MessageCount:   6,
			FieldCount:     7,
			EnumCount:      8,
			EnumValueCount: 9,
		},
		OccurredAt: occurredAt,
	})
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
	if payload.SourceArtifactChecksumSHA256 != "source-checksum" {
		t.Fatalf("source_artifact_checksum_sha256 = %q", payload.SourceArtifactChecksumSHA256)
	}
	if payload.SourceArtifactSizeBytes != 128 {
		t.Fatalf("source_artifact_size_bytes = %d", payload.SourceArtifactSizeBytes)
	}
	if payload.BufImageChecksumSHA256 != "buf-image-checksum" {
		t.Fatalf("buf_image_checksum_sha256 = %q", payload.BufImageChecksumSHA256)
	}
	if payload.BufImageSizeBytes != 256 {
		t.Fatalf("buf_image_size_bytes = %d", payload.BufImageSizeBytes)
	}
	if !payload.BufYAMLPresent {
		t.Fatalf("buf_yaml_present should be true")
	}
	if !payload.BufLockPresent {
		t.Fatalf("buf_lock_present should be true")
	}
	if payload.LintStatus != "warning" {
		t.Fatalf("lint_status = %q", payload.LintStatus)
	}
	if payload.DescriptorSummary.FileCount != 2 || payload.DescriptorSummary.EnumValueCount != 9 {
		t.Fatalf("descriptor_summary = %#v", payload.DescriptorSummary)
	}
	if !payload.OccurredAt.Equal(occurredAt) {
		t.Fatalf("occurred_at = %s, want %s", payload.OccurredAt, occurredAt)
	}

	payloadText := string(record.Payload)
	if strings.Contains(payloadText, "lint warning text") {
		t.Fatalf("lint report should not be emitted in ModuleVersionPublished payload: %s", payloadText)
	}
	if !strings.Contains(payloadText, "\"file_count\":2") {
		t.Fatalf("descriptor summary should use snake_case JSON fields: %s", payloadText)
	}
	assertNoRawTokenOrInfrastructureDetails(t, payloadText)
	assertNoRawTokenOrInfrastructureDetails(t, record.EventType)
	assertNoRawTokenOrInfrastructureDetails(t, record.DedupKey)
}

func TestNewBreakingReportCreated(t *testing.T) {
	createdAt := time.Date(2026, 6, 4, 11, 0, 0, 0, time.UTC)
	moduleName, err := domain.NewModuleName("user-api")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	version, err := domain.NewVersion("v1.0.0")
	if err != nil {
		t.Fatalf("version: %v", err)
	}

	record, err := NewBreakingReportCreated(domain.BreakingReport{
		ID:            domain.NewBreakingReportID("report-1"),
		ModuleID:      domain.NewModuleID("module-1"),
		ModuleName:    moduleName,
		BaseVersionID: domain.NewModuleVersionID("module-version-1"),
		BaseVersion:   version,
		TargetRef:     "local",
		Status:        domain.BreakingReportStatusBreaking,
		ChangeCount:   2,
		RawOutput:     "uploaded archive bytes and raw token should not be emitted",
		HumanSummary:  "summary should not be emitted",
		CreatedAt:     createdAt,
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}

	if record.EventType != EventTypeBreakingReportCreated {
		t.Fatalf("event type = %q, want %q", record.EventType, EventTypeBreakingReportCreated)
	}
	if record.DedupKey != "breaking-report:report-1:created" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}
	if record.AggregateType != aggregateTypeModule || record.AggregateID != "module-1" {
		t.Fatalf("aggregate = %s/%s", record.AggregateType, record.AggregateID)
	}

	var payload BreakingReportCreatedPayload
	if err := json.Unmarshal(record.Payload, &payload); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	if payload.ReportID != "report-1" {
		t.Fatalf("report_id = %q", payload.ReportID)
	}
	if payload.ModuleID != "module-1" {
		t.Fatalf("module_id = %q", payload.ModuleID)
	}
	if payload.ModuleName != "user-api" {
		t.Fatalf("module_name = %q", payload.ModuleName)
	}
	if payload.BaseVersionID != "module-version-1" {
		t.Fatalf("base_version_id = %q", payload.BaseVersionID)
	}
	if payload.BaseVersion != "v1.0.0" {
		t.Fatalf("base_version = %q", payload.BaseVersion)
	}
	if payload.TargetRef != "local" {
		t.Fatalf("target_ref = %q", payload.TargetRef)
	}
	if payload.Status != "breaking" {
		t.Fatalf("status = %q", payload.Status)
	}
	if payload.ChangeCount != 2 {
		t.Fatalf("change_count = %d", payload.ChangeCount)
	}
	if !payload.CreatedAt.Equal(createdAt) {
		t.Fatalf("created_at = %s, want %s", payload.CreatedAt, createdAt)
	}

	payloadText := string(record.Payload)
	for _, forbidden := range []string{"raw token", "uploaded archive", "archive bytes", "summary should not be emitted"} {
		if strings.Contains(payloadText, forbidden) {
			t.Fatalf("payload contains forbidden content %q: %s", forbidden, payloadText)
		}
	}
	assertNoRawTokenOrInfrastructureDetails(t, payloadText)
	assertNoRawTokenOrInfrastructureDetails(t, record.EventType)
	assertNoRawTokenOrInfrastructureDetails(t, record.DedupKey)
}

func TestNewModuleGitLabProjectLinked(t *testing.T) {
	occurredAt := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	moduleName, err := domain.NewModuleName("user-api")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}

	record, err := NewModuleGitLabProjectLinked(domain.ModuleGitLabProject{
		ID:                domain.NewModuleGitLabProjectID("mapping-1"),
		ModuleID:          domain.NewModuleID("module-1"),
		ModuleName:        moduleName,
		GitLabBaseURL:     "https://gitlab.example.com/",
		GitLabProjectID:   123,
		GitLabProjectPath: "platform/user-api",
	}, occurredAt)
	if err != nil {
		t.Fatalf("event: %v", err)
	}

	if record.EventType != EventTypeModuleGitLabProjectLinked {
		t.Fatalf("event type = %q, want %q", record.EventType, EventTypeModuleGitLabProjectLinked)
	}
	if record.DedupKey != "module:module-1:gitlab-project:https://gitlab.example.com:123:linked" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}
	if record.AggregateType != aggregateTypeModule || record.AggregateID != "module-1" {
		t.Fatalf("aggregate = %s/%s", record.AggregateType, record.AggregateID)
	}

	var payload ModuleGitLabProjectLinkedPayload
	if err := json.Unmarshal(record.Payload, &payload); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	if payload.ModuleID != "module-1" {
		t.Fatalf("module_id = %q", payload.ModuleID)
	}
	if payload.ModuleName != "user-api" {
		t.Fatalf("module_name = %q", payload.ModuleName)
	}
	if payload.GitLabBaseURL != "https://gitlab.example.com" {
		t.Fatalf("gitlab_base_url = %q", payload.GitLabBaseURL)
	}
	if payload.GitLabProjectID != 123 {
		t.Fatalf("gitlab_project_id = %d", payload.GitLabProjectID)
	}
	if payload.GitLabProjectPath != "platform/user-api" {
		t.Fatalf("gitlab_project_path = %q", payload.GitLabProjectPath)
	}
	if !payload.OccurredAt.Equal(occurredAt) {
		t.Fatalf("occurred_at = %s, want %s", payload.OccurredAt, occurredAt)
	}

	payloadText := string(record.Payload)
	for _, forbidden := range []string{"token", "secret", "password", "private_key"} {
		if strings.Contains(strings.ToLower(payloadText), forbidden) {
			t.Fatalf("payload contains forbidden content %q: %s", forbidden, payloadText)
		}
	}
	assertNoRawTokenOrInfrastructureDetails(t, payloadText)
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

func TestNewModuleDependenciesUpdated(t *testing.T) {
	occurredAt := time.Date(2026, 6, 4, 12, 30, 0, 0, time.UTC)
	moduleName, err := domain.NewModuleName("billing-api")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	versionValue, err := domain.NewVersion("v1.2.3")
	if err != nil {
		t.Fatalf("version: %v", err)
	}

	record, err := NewModuleDependenciesUpdated(ModuleDependenciesUpdated{
		Module: domain.Module{
			ID:   domain.NewModuleID("module-1"),
			Name: moduleName,
		},
		Version: domain.ModuleVersion{
			ID:      domain.NewModuleVersionID("module-version-1"),
			Version: versionValue,
		},
		DependencyCount:           3,
		UnresolvedDependencyCount: 2,
		OccurredAt:                occurredAt,
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}

	if record.EventType != EventTypeModuleDependenciesUpdated {
		t.Fatalf("event type = %q, want %q", record.EventType, EventTypeModuleDependenciesUpdated)
	}
	if record.DedupKey != "module:module-1:version:v1.2.3:dependencies-updated" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}
	if record.AggregateType != aggregateTypeModule || record.AggregateID != "module-1" {
		t.Fatalf("aggregate = %s/%s", record.AggregateType, record.AggregateID)
	}

	var payload ModuleDependenciesUpdatedPayload
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
	if payload.DependencyCount != 3 {
		t.Fatalf("dependency_count = %d", payload.DependencyCount)
	}
	if payload.UnresolvedDependencyCount != 2 {
		t.Fatalf("unresolved_dependency_count = %d", payload.UnresolvedDependencyCount)
	}
	if !payload.OccurredAt.Equal(occurredAt) {
		t.Fatalf("occurred_at = %s, want %s", payload.OccurredAt, occurredAt)
	}

	payloadText := string(record.Payload)
	if !strings.Contains(payloadText, "\"dependency_count\":3") || !strings.Contains(payloadText, "\"unresolved_dependency_count\":2") {
		t.Fatalf("payload should use expected snake_case count fields: %s", payloadText)
	}
	assertNoRawTokenOrInfrastructureDetails(t, payloadText)
	assertNoRawTokenOrInfrastructureDetails(t, record.EventType)
	assertNoRawTokenOrInfrastructureDetails(t, record.DedupKey)
}
