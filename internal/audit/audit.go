package audit

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

const MaxPayloadBytes = 16 * 1024

const (
	OutcomeSuccess = "success"
	OutcomeFailure = "failure"
)

type AuditResource struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AuditEvent struct {
	ID               string          `json:"id,omitempty"`
	Type             string          `json:"type"`
	Actor            string          `json:"actor"`
	Action           string          `json:"action"`
	Outcome          string          `json:"outcome"`
	Resource         AuditResource   `json:"resource"`
	RelatedResources []AuditResource `json:"related_resources,omitempty"`
	Payload          json.RawMessage `json:"payload"`
	OccurredAt       time.Time       `json:"occurred_at"`
}

type AuditSink interface {
	Record(ctx context.Context, event AuditEvent) error
}

var ErrPayloadTooLarge = errors.New("audit payload too large")

type CommunityAuditSink struct {
	repository domain.GovernanceAuditRepository
}

func NewCommunityAuditSink(repository domain.GovernanceAuditRepository) *CommunityAuditSink {
	return &CommunityAuditSink{repository: repository}
}

func (sink *CommunityAuditSink) Record(ctx context.Context, event AuditEvent) error {
	if len(event.Payload) > MaxPayloadBytes {
		return ErrPayloadTooLarge
	}
	eventType, err := domain.NewGovernanceAuditEventType(event.Type)
	if err != nil {
		return err
	}
	payload := event.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	stored := domain.GovernanceAuditEvent{
		ID:          domain.NewGovernanceAuditEventID(event.ID),
		EventType:   eventType,
		Actor:       event.Actor,
		PayloadJSON: payload,
		CreatedAt:   event.OccurredAt,
	}
	for _, resource := range append([]AuditResource{event.Resource}, event.RelatedResources...) {
		applyResource(&stored, resource)
	}
	return sink.repository.Append(ctx, stored)
}

func applyResource(stored *domain.GovernanceAuditEvent, resource AuditResource) {
	switch resource.Type {
	case "module":
		if resource.ID != "" {
			moduleID := domain.NewModuleID(resource.ID)
			stored.ModuleID = &moduleID
		}
		if resource.Name != "" {
			moduleName, err := domain.NewModuleName(resource.Name)
			if err == nil {
				stored.ModuleName = moduleName
			}
		}
	case "approval_request":
		if resource.ID != "" {
			requestID := domain.NewApprovalRequestID(resource.ID)
			stored.ApprovalRequestID = &requestID
		}
	case "breaking_report":
		if resource.ID != "" {
			reportID := domain.NewBreakingReportID(resource.ID)
			stored.BreakingReportID = &reportID
		}
	}
}
