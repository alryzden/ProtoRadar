package httptransport

import (
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type breakingChangeDTO struct {
	Category    string `json:"category"`
	FilePath    string `json:"file_path"`
	PackageName string `json:"package_name"`
	Symbol      string `json:"symbol"`
	RuleID      string `json:"rule_id"`
	Message     string `json:"message"`
	Severity    string `json:"severity"`
}

type breakingReportDTO struct {
	ID           string              `json:"id"`
	Module       string              `json:"module"`
	Against      string              `json:"against"`
	TargetRef    string              `json:"target_ref"`
	Status       string              `json:"status"`
	ChangeCount  int                 `json:"change_count"`
	Changes      []breakingChangeDTO `json:"changes"`
	HumanSummary string              `json:"human_summary"`
	CreatedAt    time.Time           `json:"created_at"`
}

type breakingReportSummaryDTO struct {
	ID           string    `json:"id"`
	Module       string    `json:"module"`
	Against      string    `json:"against"`
	TargetRef    string    `json:"target_ref"`
	Status       string    `json:"status"`
	ChangeCount  int       `json:"change_count"`
	HumanSummary string    `json:"human_summary"`
	CreatedAt    time.Time `json:"created_at"`
}

type listBreakingReportsResponse struct {
	Reports []breakingReportSummaryDTO `json:"reports"`
}

func breakingReportResponse(report domain.BreakingReport, changes []domain.BreakingChange) breakingReportDTO {
	items := make([]breakingChangeDTO, 0, len(changes))
	for _, change := range changes {
		items = append(items, breakingChangeResponse(change))
	}
	return breakingReportDTO{
		ID:           report.ID.String(),
		Module:       report.ModuleName.String(),
		Against:      report.BaseVersion.String(),
		TargetRef:    report.TargetRef,
		Status:       report.Status.String(),
		ChangeCount:  report.ChangeCount,
		Changes:      items,
		HumanSummary: report.HumanSummary,
		CreatedAt:    report.CreatedAt,
	}
}

func breakingReportSummaryResponse(report domain.BreakingReport) breakingReportSummaryDTO {
	return breakingReportSummaryDTO{
		ID:           report.ID.String(),
		Module:       report.ModuleName.String(),
		Against:      report.BaseVersion.String(),
		TargetRef:    report.TargetRef,
		Status:       report.Status.String(),
		ChangeCount:  report.ChangeCount,
		HumanSummary: report.HumanSummary,
		CreatedAt:    report.CreatedAt,
	}
}

func breakingChangeResponse(change domain.BreakingChange) breakingChangeDTO {
	return breakingChangeDTO{
		Category:    change.Category,
		FilePath:    change.FilePath,
		PackageName: change.PackageName,
		Symbol:      change.Symbol,
		RuleID:      change.RuleID,
		Message:     change.Message,
		Severity:    change.Severity,
	}
}
