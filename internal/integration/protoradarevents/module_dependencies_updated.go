package protoradarevents

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/outbox"
)

const EventTypeModuleDependenciesUpdated = "protoradar.module_dependencies.updated"

type ModuleDependenciesUpdated struct {
	Module                    domain.Module
	Version                   domain.ModuleVersion
	DependencyCount           int
	UnresolvedDependencyCount int
	OccurredAt                time.Time
}

type ModuleDependenciesUpdatedPayload struct {
	ModuleID                  string    `json:"module_id"`
	ModuleName                string    `json:"module_name"`
	ModuleVersionID           string    `json:"module_version_id"`
	Version                   string    `json:"version"`
	DependencyCount           int       `json:"dependency_count"`
	UnresolvedDependencyCount int       `json:"unresolved_dependency_count"`
	OccurredAt                time.Time `json:"occurred_at"`
}

func NewModuleDependenciesUpdated(event ModuleDependenciesUpdated) (outbox.Record, error) {
	payload := ModuleDependenciesUpdatedPayload{
		ModuleID:                  event.Module.ID.String(),
		ModuleName:                event.Module.Name.String(),
		ModuleVersionID:           event.Version.ID.String(),
		Version:                   event.Version.Version.String(),
		DependencyCount:           event.DependencyCount,
		UnresolvedDependencyCount: event.UnresolvedDependencyCount,
		OccurredAt:                event.OccurredAt,
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
		EventType:     EventTypeModuleDependenciesUpdated,
		DedupKey:      fmt.Sprintf("module:%s:version:%s:dependencies-updated", moduleID, version),
		Payload:       body,
		OccurredAt:    event.OccurredAt,
	}, nil
}
