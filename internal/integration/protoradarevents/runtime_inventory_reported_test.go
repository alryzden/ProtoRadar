package protoradarevents

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

func TestNewRuntimeInventoryReported(t *testing.T) {
	occurredAt := time.Date(2026, 6, 4, 13, 0, 0, 0, time.UTC)
	serviceName, err := domain.NewRuntimeServiceName("billing-service")
	if err != nil {
		t.Fatalf("service name: %v", err)
	}
	environment, err := domain.NewRuntimeEnvironment("prod")
	if err != nil {
		t.Fatalf("environment: %v", err)
	}

	record, err := NewRuntimeInventoryReported(RuntimeInventoryReported{
		DeploymentID:     domain.NewRuntimeDeploymentID("deployment-1"),
		ServiceID:        domain.NewRuntimeServiceID("service-1"),
		ServiceName:      serviceName,
		Environment:      environment,
		GitCommit:        "abc123",
		BuildVersion:     "pipeline-456",
		ModuleUsageCount: 7,
		DriftCounts: RuntimeInventoryDriftCounts{
			UpToDate:          3,
			BehindLatest:      2,
			UnknownVersion:    1,
			DeprecatedVersion: 1,
		},
		OccurredAt: occurredAt,
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}

	if record.EventType != EventTypeRuntimeInventoryReported {
		t.Fatalf("event type = %q, want %q", record.EventType, EventTypeRuntimeInventoryReported)
	}
	if record.DedupKey != "runtime-deployment:deployment-1:reported" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}
	if record.AggregateType != aggregateTypeRuntimeDeployment || record.AggregateID != "deployment-1" {
		t.Fatalf("aggregate = %s/%s", record.AggregateType, record.AggregateID)
	}
	if !record.OccurredAt.Equal(occurredAt) {
		t.Fatalf("occurred_at = %s, want %s", record.OccurredAt, occurredAt)
	}

	var payload RuntimeInventoryReportedPayload
	if err := json.Unmarshal(record.Payload, &payload); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	if payload.DeploymentID != "deployment-1" {
		t.Fatalf("deployment_id = %q", payload.DeploymentID)
	}
	if payload.ServiceID != "service-1" {
		t.Fatalf("service_id = %q", payload.ServiceID)
	}
	if payload.ServiceName != "billing-service" {
		t.Fatalf("service_name = %q", payload.ServiceName)
	}
	if payload.Environment != "prod" {
		t.Fatalf("environment = %q", payload.Environment)
	}
	if payload.GitCommit != "abc123" {
		t.Fatalf("git_commit = %q", payload.GitCommit)
	}
	if payload.BuildVersion != "pipeline-456" {
		t.Fatalf("build_version = %q", payload.BuildVersion)
	}
	if payload.ModuleUsageCount != 7 {
		t.Fatalf("module_usage_count = %d", payload.ModuleUsageCount)
	}
	if payload.DriftCounts.UpToDate != 3 || payload.DriftCounts.BehindLatest != 2 || payload.DriftCounts.UnknownVersion != 1 || payload.DriftCounts.DeprecatedVersion != 1 {
		t.Fatalf("drift_counts = %#v", payload.DriftCounts)
	}
	if !payload.OccurredAt.Equal(occurredAt) {
		t.Fatalf("occurred_at = %s, want %s", payload.OccurredAt, occurredAt)
	}

	payloadText := string(record.Payload)
	for _, expected := range []string{
		"\"module_usage_count\":7",
		"\"up_to_date\":3",
		"\"behind_latest\":2",
		"\"unknown_version\":1",
		"\"deprecated_version\":1",
	} {
		if !strings.Contains(payloadText, expected) {
			t.Fatalf("payload missing %s: %s", expected, payloadText)
		}
	}
	for _, forbidden := range []string{
		"raw_token",
		"api_token_value",
		"secret",
		"bearer",
		"password",
		"private_key",
		"modules",
		"module_usages",
	} {
		if strings.Contains(strings.ToLower(payloadText), forbidden) {
			t.Fatalf("payload contains forbidden detail %q: %s", forbidden, payloadText)
		}
	}
	assertNoRawTokenOrInfrastructureDetails(t, payloadText)
	assertNoRawTokenOrInfrastructureDetails(t, record.EventType)
	assertNoRawTokenOrInfrastructureDetails(t, record.DedupKey)
}
