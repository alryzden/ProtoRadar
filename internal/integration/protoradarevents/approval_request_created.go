package protoradarevents

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/outbox"
)

const (
	EventTypeApprovalRequestCreated = "protoradar.approval_request.created"

	aggregateTypeApprovalRequest = "approval_request"
)

type ApprovalRequestCreatedPayload struct {
	ApprovalRequestID string    `json:"approval_request_id"`
	ModuleID          string    `json:"module_id"`
	ModuleName        string    `json:"module_name"`
	BreakingReportID  string    `json:"breaking_report_id,omitempty"`
	TargetRef         string    `json:"target_ref"`
	Status            string    `json:"status"`
	RequiredApprovals int       `json:"required_approvals"`
	ReceivedApprovals int       `json:"received_approvals"`
	Actor             string    `json:"actor,omitempty"`
	OccurredAt        time.Time `json:"occurred_at"`
}

type ApprovalRequestCreated struct {
	Request    domain.ApprovalRequest
	Actor      string
	OccurredAt time.Time
}

func NewApprovalRequestCreated(event ApprovalRequestCreated) (outbox.Record, error) {
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = event.Request.CreatedAt
	}
	payload := ApprovalRequestCreatedPayload{
		ApprovalRequestID: event.Request.ID.String(),
		ModuleID:          event.Request.ModuleID.String(),
		ModuleName:        event.Request.ModuleName.String(),
		BreakingReportID:  breakingReportIDString(event.Request.BreakingReportID),
		TargetRef:         event.Request.TargetRef,
		Status:            event.Request.Status.String(),
		RequiredApprovals: event.Request.RequiredApprovals,
		ReceivedApprovals: event.Request.ReceivedApprovals,
		Actor:             event.Actor,
		OccurredAt:        occurredAt,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return outbox.Record{}, err
	}

	requestID := event.Request.ID.String()
	return outbox.Record{
		AggregateType: aggregateTypeApprovalRequest,
		AggregateID:   requestID,
		EventType:     EventTypeApprovalRequestCreated,
		DedupKey:      fmt.Sprintf("approval-request:%s:created", requestID),
		Payload:       body,
		OccurredAt:    occurredAt,
	}, nil
}

func breakingReportIDString(id *domain.BreakingReportID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
