package protoradarevents

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/outbox"
)

const (
	EventTypeRuntimeInventoryReported = "protoradar.runtime_inventory.reported"

	aggregateTypeRuntimeDeployment = "runtime_deployment"
)

type RuntimeInventoryReported struct {
	DeploymentID     domain.RuntimeDeploymentID
	ServiceID        domain.RuntimeServiceID
	ServiceName      domain.RuntimeServiceName
	Environment      domain.RuntimeEnvironment
	GitCommit        string
	BuildVersion     string
	ModuleUsageCount int
	DriftCounts      RuntimeInventoryDriftCounts
	OccurredAt       time.Time
}

type RuntimeInventoryDriftCounts struct {
	UpToDate          int
	BehindLatest      int
	UnknownVersion    int
	DeprecatedVersion int
}

type RuntimeInventoryReportedPayload struct {
	DeploymentID     string                             `json:"deployment_id"`
	ServiceID        string                             `json:"service_id"`
	ServiceName      string                             `json:"service_name"`
	Environment      string                             `json:"environment"`
	GitCommit        string                             `json:"git_commit"`
	BuildVersion     string                             `json:"build_version"`
	ModuleUsageCount int                                `json:"module_usage_count"`
	DriftCounts      RuntimeInventoryDriftCountsPayload `json:"drift_counts"`
	OccurredAt       time.Time                          `json:"occurred_at"`
}

type RuntimeInventoryDriftCountsPayload struct {
	UpToDate          int `json:"up_to_date"`
	BehindLatest      int `json:"behind_latest"`
	UnknownVersion    int `json:"unknown_version"`
	DeprecatedVersion int `json:"deprecated_version"`
}

func NewRuntimeInventoryReported(event RuntimeInventoryReported) (outbox.Record, error) {
	payload := RuntimeInventoryReportedPayload{
		DeploymentID:     event.DeploymentID.String(),
		ServiceID:        event.ServiceID.String(),
		ServiceName:      event.ServiceName.String(),
		Environment:      event.Environment.String(),
		GitCommit:        event.GitCommit,
		BuildVersion:     event.BuildVersion,
		ModuleUsageCount: event.ModuleUsageCount,
		DriftCounts: RuntimeInventoryDriftCountsPayload{
			UpToDate:          event.DriftCounts.UpToDate,
			BehindLatest:      event.DriftCounts.BehindLatest,
			UnknownVersion:    event.DriftCounts.UnknownVersion,
			DeprecatedVersion: event.DriftCounts.DeprecatedVersion,
		},
		OccurredAt: event.OccurredAt,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return outbox.Record{}, err
	}

	deploymentID := event.DeploymentID.String()
	return outbox.Record{
		AggregateType: aggregateTypeRuntimeDeployment,
		AggregateID:   deploymentID,
		EventType:     EventTypeRuntimeInventoryReported,
		DedupKey:      fmt.Sprintf("runtime-deployment:%s:reported", deploymentID),
		Payload:       body,
		OccurredAt:    event.OccurredAt,
	}, nil
}
