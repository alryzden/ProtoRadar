package protoradarevents

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/outbox"
)

const EventTypeApprovalDecisionRecorded = "protoradar.approval_decision.recorded"

type ApprovalDecisionRecordedPayload struct {
	ApprovalDecisionID string    `json:"approval_decision_id"`
	ApprovalRequestID  string    `json:"approval_request_id"`
	RequirementID      string    `json:"requirement_id"`
	Decision           string    `json:"decision"`
	DecidedBy          string    `json:"decided_by"`
	ModuleID           string    `json:"module_id,omitempty"`
	ModuleName         string    `json:"module_name,omitempty"`
	BreakingReportID   string    `json:"breaking_report_id,omitempty"`
	OccurredAt         time.Time `json:"occurred_at"`
}

type ApprovalDecisionRecorded struct {
	Decision         domain.ApprovalDecision
	ModuleID         domain.ModuleID
	ModuleName       domain.ModuleName
	BreakingReportID *domain.BreakingReportID
	OccurredAt       time.Time
}

func NewApprovalDecisionRecorded(event ApprovalDecisionRecorded) (outbox.Record, error) {
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = event.Decision.CreatedAt
	}
	payload := ApprovalDecisionRecordedPayload{
		ApprovalDecisionID: event.Decision.ID.String(),
		ApprovalRequestID:  event.Decision.ApprovalRequestID.String(),
		RequirementID:      event.Decision.RequirementID.String(),
		Decision:           event.Decision.Decision.String(),
		DecidedBy:          event.Decision.DecidedBy,
		ModuleID:           event.ModuleID.String(),
		ModuleName:         event.ModuleName.String(),
		BreakingReportID:   breakingReportIDString(event.BreakingReportID),
		OccurredAt:         occurredAt,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return outbox.Record{}, err
	}

	requestID := event.Decision.ApprovalRequestID.String()
	decisionID := event.Decision.ID.String()
	return outbox.Record{
		AggregateType: aggregateTypeApprovalRequest,
		AggregateID:   requestID,
		EventType:     EventTypeApprovalDecisionRecorded,
		DedupKey:      fmt.Sprintf("approval-decision:%s:recorded", decisionID),
		Payload:       body,
		OccurredAt:    occurredAt,
	}, nil
}
