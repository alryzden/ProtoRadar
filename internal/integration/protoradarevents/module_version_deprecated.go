package protoradarevents

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/outbox"
)

const EventTypeModuleVersionDeprecated = "protoradar.module_version.deprecated"

type ModuleVersionDeprecated struct {
	Module            domain.Module
	Version           domain.ModuleVersion
	DeprecatedBy      string
	DeprecationReason string
	OccurredAt        time.Time
}

type ModuleVersionDeprecatedPayload struct {
	ModuleID          string    `json:"module_id"`
	ModuleName        string    `json:"module_name"`
	ModuleVersionID   string    `json:"module_version_id"`
	Version           string    `json:"version"`
	DeprecatedBy      string    `json:"deprecated_by"`
	DeprecationReason string    `json:"deprecation_reason"`
	OccurredAt        time.Time `json:"occurred_at"`
}

func NewModuleVersionDeprecated(event ModuleVersionDeprecated) (outbox.Record, error) {
	payload := ModuleVersionDeprecatedPayload{
		ModuleID:          event.Module.ID.String(),
		ModuleName:        event.Module.Name.String(),
		ModuleVersionID:   event.Version.ID.String(),
		Version:           event.Version.Version.String(),
		DeprecatedBy:      event.DeprecatedBy,
		DeprecationReason: event.DeprecationReason,
		OccurredAt:        event.OccurredAt,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return outbox.Record{}, err
	}

	moduleID := event.Module.ID.String()
	version := event.Version.Version.String()
	return outbox.Record{
		AggregateType: aggregateTypeModule,
		AggregateID:   moduleID,
		EventType:     EventTypeModuleVersionDeprecated,
		DedupKey:      fmt.Sprintf("module:%s:version:%s:deprecated", moduleID, version),
		Payload:       body,
		OccurredAt:    event.OccurredAt,
	}, nil
}
