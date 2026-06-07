package audit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

func TestCommunityAuditSinkRecordsGovernanceAuditEvent(t *testing.T) {
	repo := &fakeGovernanceAuditRepository{}
	sink := NewCommunityAuditSink(repo)

	err := sink.Record(context.Background(), AuditEvent{
		ID:       "audit-1",
		Type:     domain.GovernanceAuditEventTypeApprovalRequestCreated.String(),
		Actor:    "alice",
		Action:   "approval_request.create",
		Outcome:  OutcomeSuccess,
		Resource: AuditResource{Type: "approval_request", ID: "request-1"},
		RelatedResources: []AuditResource{
			{Type: "module", ID: "module-1", Name: "user-api"},
			{Type: "breaking_report", ID: "report-1"},
		},
		Payload:    json.RawMessage(`{"requirement_count":1}`),
		OccurredAt: time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if len(repo.events) != 1 {
		t.Fatalf("events = %d, want 1", len(repo.events))
	}
	event := repo.events[0]
	if event.ID != "audit-1" || event.EventType != domain.GovernanceAuditEventTypeApprovalRequestCreated || event.Actor != "alice" {
		t.Fatalf("event = %#v", event)
	}
	if event.ModuleID == nil || *event.ModuleID != "module-1" || event.ModuleName.String() != "user-api" {
		t.Fatalf("module resource = %#v", event)
	}
	if event.ApprovalRequestID == nil || *event.ApprovalRequestID != "request-1" {
		t.Fatalf("approval request resource = %#v", event)
	}
	if event.BreakingReportID == nil || *event.BreakingReportID != "report-1" {
		t.Fatalf("breaking report resource = %#v", event)
	}
}

func TestCommunityAuditSinkRejectsOversizedPayload(t *testing.T) {
	repo := &fakeGovernanceAuditRepository{}
	sink := NewCommunityAuditSink(repo)

	err := sink.Record(context.Background(), AuditEvent{
		ID:      "audit-1",
		Type:    domain.GovernanceAuditEventTypeApprovalRequestCreated.String(),
		Payload: json.RawMessage(strings.Repeat("x", MaxPayloadBytes+1)),
	})
	if !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("error = %v, want ErrPayloadTooLarge", err)
	}
	if len(repo.events) != 0 {
		t.Fatalf("events = %#v", repo.events)
	}
}

func TestCommunityAuditSinkRejectsUnknownGovernanceAuditEventType(t *testing.T) {
	repo := &fakeGovernanceAuditRepository{}
	sink := NewCommunityAuditSink(repo)

	err := sink.Record(context.Background(), AuditEvent{
		ID:      "audit-1",
		Type:    "manual_override",
		Payload: json.RawMessage(`{}`),
	})
	if !errors.Is(err, domain.ErrInvalidGovernanceAuditEventType) {
		t.Fatalf("error = %v, want ErrInvalidGovernanceAuditEventType", err)
	}
	if len(repo.events) != 0 {
		t.Fatalf("events = %#v", repo.events)
	}
}

type fakeGovernanceAuditRepository struct {
	events []domain.GovernanceAuditEvent
}

func (repo *fakeGovernanceAuditRepository) Append(ctx context.Context, event domain.GovernanceAuditEvent) error {
	repo.events = append(repo.events, event)
	return nil
}

func (repo *fakeGovernanceAuditRepository) ListByApprovalRequest(ctx context.Context, approvalRequestID domain.ApprovalRequestID, limit int, offset int) ([]domain.GovernanceAuditEvent, error) {
	return nil, nil
}

func (repo *fakeGovernanceAuditRepository) ListByModule(ctx context.Context, moduleID domain.ModuleID, limit int, offset int) ([]domain.GovernanceAuditEvent, error) {
	return nil, nil
}
