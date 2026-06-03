package protoradarevents

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/outbox"
)

const (
	EventTypeModuleCreated = "protoradar.module.created"

	aggregateTypeModule = "module"
)

type ModuleCreatedPayload struct {
	ModuleID      string    `json:"module_id"`
	ModuleName    string    `json:"module_name"`
	RepositoryURL string    `json:"repository_url"`
	OccurredAt    time.Time `json:"occurred_at"`
}

func NewModuleCreated(module domain.Module, occurredAt time.Time) (outbox.Record, error) {
	payload := ModuleCreatedPayload{
		ModuleID:      module.ID.String(),
		ModuleName:    module.Name.String(),
		RepositoryURL: module.RepositoryURL,
		OccurredAt:    occurredAt,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return outbox.Record{}, err
	}

	moduleID := module.ID.String()
	return outbox.Record{
		AggregateType: aggregateTypeModule,
		AggregateID:   moduleID,
		EventType:     EventTypeModuleCreated,
		DedupKey:      fmt.Sprintf("module:%s:created", moduleID),
		Payload:       body,
		OccurredAt:    occurredAt,
	}, nil
}
