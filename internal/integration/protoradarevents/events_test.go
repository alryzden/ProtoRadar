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

func TestNewApprovalRequestCreated(t *testing.T) {
	occurredAt := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	moduleName, err := domain.NewModuleName("user-api")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	reportID := domain.NewBreakingReportID("report-1")

	record, err := NewApprovalRequestCreated(ApprovalRequestCreated{
		Request: domain.ApprovalRequest{
			ID:                domain.NewApprovalRequestID("approval-request-1"),
			ModuleID:          domain.NewModuleID("module-1"),
			ModuleName:        moduleName,
			BreakingReportID:  &reportID,
			TargetRef:         "feature/change",
			Status:            domain.ApprovalRequestStatusPending,
			RequiredApprovals: 2,
			ReceivedApprovals: 0,
			CreatedAt:         occurredAt,
		},
		Actor:      "alice",
		OccurredAt: occurredAt,
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}

	if record.EventType != EventTypeApprovalRequestCreated {
		t.Fatalf("event type = %q, want %q", record.EventType, EventTypeApprovalRequestCreated)
	}
	if record.DedupKey != "approval-request:approval-request-1:created" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}
	if record.AggregateType != aggregateTypeApprovalRequest || record.AggregateID != "approval-request-1" {
		t.Fatalf("aggregate = %s/%s", record.AggregateType, record.AggregateID)
	}

	var payload ApprovalRequestCreatedPayload
	if err := json.Unmarshal(record.Payload, &payload); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	if payload.ApprovalRequestID != "approval-request-1" {
		t.Fatalf("approval_request_id = %q", payload.ApprovalRequestID)
	}
	if payload.ModuleID != "module-1" || payload.ModuleName != "user-api" {
		t.Fatalf("module = %s/%s", payload.ModuleID, payload.ModuleName)
	}
	if payload.BreakingReportID != "report-1" {
		t.Fatalf("breaking_report_id = %q", payload.BreakingReportID)
	}
	if payload.TargetRef != "feature/change" {
		t.Fatalf("target_ref = %q", payload.TargetRef)
	}
	if payload.Status != "pending" || payload.RequiredApprovals != 2 || payload.ReceivedApprovals != 0 {
		t.Fatalf("status/counts = %s/%d/%d", payload.Status, payload.RequiredApprovals, payload.ReceivedApprovals)
	}
	if payload.Actor != "alice" {
		t.Fatalf("actor = %q", payload.Actor)
	}
	if !payload.OccurredAt.Equal(occurredAt) {
		t.Fatalf("occurred_at = %s, want %s", payload.OccurredAt, occurredAt)
	}

	payloadText := string(record.Payload)
	if !strings.Contains(payloadText, "\"approval_request_id\":\"approval-request-1\"") {
		t.Fatalf("payload should use expected snake_case fields: %s", payloadText)
	}
	assertNoRawTokenOrInfrastructureDetails(t, payloadText)
	assertNoRawTokenOrInfrastructureDetails(t, record.EventType)
	assertNoRawTokenOrInfrastructureDetails(t, record.DedupKey)
}

func TestNewApprovalDecisionRecorded(t *testing.T) {
	occurredAt := time.Date(2026, 6, 5, 10, 30, 0, 0, time.UTC)
	moduleName, err := domain.NewModuleName("billing-api")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	reportID := domain.NewBreakingReportID("report-2")

	record, err := NewApprovalDecisionRecorded(ApprovalDecisionRecorded{
		Decision: domain.ApprovalDecision{
			ID:                domain.NewApprovalDecisionID("approval-decision-1"),
			ApprovalRequestID: domain.NewApprovalRequestID("approval-request-2"),
			RequirementID:     domain.NewApprovalRequirementID("requirement-1"),
			Decision:          domain.ApprovalDecisionValueApproved,
			DecidedBy:         "team/platform",
			Comment:           "comment should not be emitted",
			CreatedAt:         occurredAt,
		},
		ModuleID:         domain.NewModuleID("module-2"),
		ModuleName:       moduleName,
		BreakingReportID: &reportID,
		OccurredAt:       occurredAt,
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}

	if record.EventType != EventTypeApprovalDecisionRecorded {
		t.Fatalf("event type = %q, want %q", record.EventType, EventTypeApprovalDecisionRecorded)
	}
	if record.DedupKey != "approval-decision:approval-decision-1:recorded" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}
	if record.AggregateType != aggregateTypeApprovalRequest || record.AggregateID != "approval-request-2" {
		t.Fatalf("aggregate = %s/%s", record.AggregateType, record.AggregateID)
	}

	var payload ApprovalDecisionRecordedPayload
	if err := json.Unmarshal(record.Payload, &payload); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	if payload.ApprovalDecisionID != "approval-decision-1" {
		t.Fatalf("approval_decision_id = %q", payload.ApprovalDecisionID)
	}
	if payload.ApprovalRequestID != "approval-request-2" || payload.RequirementID != "requirement-1" {
		t.Fatalf("request/requirement = %s/%s", payload.ApprovalRequestID, payload.RequirementID)
	}
	if payload.Decision != "approved" || payload.DecidedBy != "team/platform" {
		t.Fatalf("decision = %s by %s", payload.Decision, payload.DecidedBy)
	}
	if payload.ModuleID != "module-2" || payload.ModuleName != "billing-api" {
		t.Fatalf("module = %s/%s", payload.ModuleID, payload.ModuleName)
	}
	if payload.BreakingReportID != "report-2" {
		t.Fatalf("breaking_report_id = %q", payload.BreakingReportID)
	}
	if !payload.OccurredAt.Equal(occurredAt) {
		t.Fatalf("occurred_at = %s, want %s", payload.OccurredAt, occurredAt)
	}

	payloadText := string(record.Payload)
	for _, forbidden := range []string{"comment should not be emitted", "token", "secret", "password"} {
		if strings.Contains(strings.ToLower(payloadText), forbidden) {
			t.Fatalf("payload contains forbidden content %q: %s", forbidden, payloadText)
		}
	}
	assertNoRawTokenOrInfrastructureDetails(t, payloadText)
	assertNoRawTokenOrInfrastructureDetails(t, record.EventType)
	assertNoRawTokenOrInfrastructureDetails(t, record.DedupKey)
}

func TestNewModuleOwnerAdded(t *testing.T) {
	occurredAt := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	moduleName, err := domain.NewModuleName("user-api")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	owner := domain.ModuleOwner{
		ID:          domain.NewModuleOwnerID("owner-1"),
		ModuleID:    domain.NewModuleID("module-1"),
		ModuleName:  moduleName,
		SubjectType: domain.GovernanceSubjectTypeUser,
		Subject:     "alice",
		Role:        domain.ModuleOwnerRoleOwner,
		CreatedAt:   occurredAt,
	}

	record, err := NewModuleOwnerAdded(ModuleOwnerAdded{Owner: owner, Actor: "admin", OccurredAt: occurredAt})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	if record.EventType != EventTypeModuleOwnerAdded {
		t.Fatalf("event type = %q, want %q", record.EventType, EventTypeModuleOwnerAdded)
	}
	if record.DedupKey != "module:module-1:owner:owner-1:added" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}
	if record.AggregateType != aggregateTypeModule || record.AggregateID != "module-1" {
		t.Fatalf("aggregate = %s/%s", record.AggregateType, record.AggregateID)
	}
	var payload ModuleOwnerAddedPayload
	if err := json.Unmarshal(record.Payload, &payload); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	if payload.OwnerID != "owner-1" || payload.ModuleName != "user-api" || payload.SubjectType != "user" || payload.Subject != "alice" || payload.Role != "owner" || payload.Actor != "admin" {
		t.Fatalf("payload = %#v", payload)
	}
	if !payload.OccurredAt.Equal(occurredAt) {
		t.Fatalf("occurred_at = %s, want %s", payload.OccurredAt, occurredAt)
	}
	assertNoRawTokenOrInfrastructureDetails(t, string(record.Payload))
	assertNoRawTokenOrInfrastructureDetails(t, record.EventType)
	assertNoRawTokenOrInfrastructureDetails(t, record.DedupKey)
}

func TestNewModuleOwnerRemoved(t *testing.T) {
	occurredAt := time.Date(2026, 6, 5, 12, 30, 0, 0, time.UTC)
	moduleName, err := domain.NewModuleName("billing-api")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	owner := domain.ModuleOwner{
		ID:          domain.NewModuleOwnerID("owner-2"),
		ModuleID:    domain.NewModuleID("module-2"),
		ModuleName:  moduleName,
		SubjectType: domain.GovernanceSubjectTypeTeam,
		Subject:     "platform",
		Role:        domain.ModuleOwnerRoleMaintainer,
		UpdatedAt:   occurredAt,
	}

	record, err := NewModuleOwnerRemoved(ModuleOwnerRemoved{Owner: owner, Actor: "admin", OccurredAt: occurredAt})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	if record.EventType != EventTypeModuleOwnerRemoved {
		t.Fatalf("event type = %q, want %q", record.EventType, EventTypeModuleOwnerRemoved)
	}
	if record.DedupKey != "module:module-2:owner:owner-2:removed" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}
	if record.AggregateType != aggregateTypeModule || record.AggregateID != "module-2" {
		t.Fatalf("aggregate = %s/%s", record.AggregateType, record.AggregateID)
	}
	var payload ModuleOwnerRemovedPayload
	if err := json.Unmarshal(record.Payload, &payload); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	if payload.OwnerID != "owner-2" || payload.ModuleName != "billing-api" || payload.SubjectType != "team" || payload.Subject != "platform" || payload.Role != "maintainer" || payload.Actor != "admin" {
		t.Fatalf("payload = %#v", payload)
	}
	if !payload.OccurredAt.Equal(occurredAt) {
		t.Fatalf("occurred_at = %s, want %s", payload.OccurredAt, occurredAt)
	}
	assertNoRawTokenOrInfrastructureDetails(t, string(record.Payload))
	assertNoRawTokenOrInfrastructureDetails(t, record.EventType)
	assertNoRawTokenOrInfrastructureDetails(t, record.DedupKey)
}
