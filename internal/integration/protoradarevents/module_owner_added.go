package protoradarevents

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/outbox"
)

const EventTypeModuleOwnerAdded = "protoradar.module_owner.added"

type ModuleOwnerAddedPayload struct {
	OwnerID     string    `json:"owner_id"`
	ModuleID    string    `json:"module_id"`
	ModuleName  string    `json:"module_name"`
	SubjectType string    `json:"subject_type"`
	Subject     string    `json:"subject"`
	Role        string    `json:"role"`
	Actor       string    `json:"actor,omitempty"`
	OccurredAt  time.Time `json:"occurred_at"`
}

type ModuleOwnerAdded struct {
	Owner      domain.ModuleOwner
	Actor      string
	OccurredAt time.Time
}

func NewModuleOwnerAdded(event ModuleOwnerAdded) (outbox.Record, error) {
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = event.Owner.CreatedAt
	}
	payload := ModuleOwnerAddedPayload{
		OwnerID:     event.Owner.ID.String(),
		ModuleID:    event.Owner.ModuleID.String(),
		ModuleName:  event.Owner.ModuleName.String(),
		SubjectType: event.Owner.SubjectType.String(),
		Subject:     event.Owner.Subject,
		Role:        event.Owner.Role.String(),
		Actor:       event.Actor,
		OccurredAt:  occurredAt,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return outbox.Record{}, err
	}

	moduleID := event.Owner.ModuleID.String()
	ownerID := event.Owner.ID.String()
	return outbox.Record{
		AggregateType: aggregateTypeModule,
		AggregateID:   moduleID,
		EventType:     EventTypeModuleOwnerAdded,
		DedupKey:      fmt.Sprintf("module:%s:owner:%s:added", moduleID, ownerID),
		Payload:       body,
		OccurredAt:    occurredAt,
	}, nil
}
