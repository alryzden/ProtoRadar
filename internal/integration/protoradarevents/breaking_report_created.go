package protoradarevents

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/outbox"
)

const EventTypeBreakingReportCreated = "protoradar.breaking_report.created"

type BreakingReportCreatedPayload struct {
	ReportID      string    `json:"report_id"`
	ModuleID      string    `json:"module_id"`
	ModuleName    string    `json:"module_name"`
	BaseVersionID string    `json:"base_version_id"`
	BaseVersion   string    `json:"base_version"`
	TargetRef     string    `json:"target_ref"`
	Status        string    `json:"status"`
	ChangeCount   int       `json:"change_count"`
	CreatedAt     time.Time `json:"created_at"`
}

func NewBreakingReportCreated(report domain.BreakingReport) (outbox.Record, error) {
	payload := BreakingReportCreatedPayload{
		ReportID:      report.ID.String(),
		ModuleID:      report.ModuleID.String(),
		ModuleName:    report.ModuleName.String(),
		BaseVersionID: report.BaseVersionID.String(),
		BaseVersion:   report.BaseVersion.String(),
		TargetRef:     report.TargetRef,
		Status:        report.Status.String(),
		ChangeCount:   report.ChangeCount,
		CreatedAt:     report.CreatedAt,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return outbox.Record{}, err
	}

	return outbox.Record{
		AggregateType: aggregateTypeModule,
		AggregateID:   report.ModuleID.String(),
		EventType:     EventTypeBreakingReportCreated,
		DedupKey:      fmt.Sprintf("breaking-report:%s:created", report.ID.String()),
		Payload:       body,
		OccurredAt:    report.CreatedAt,
	}, nil
}
