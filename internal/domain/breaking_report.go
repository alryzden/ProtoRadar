package domain

import (
	"strings"
	"time"
)

type BreakingReportStatus string

const (
	BreakingReportStatusPassed   BreakingReportStatus = "passed"
	BreakingReportStatusBreaking BreakingReportStatus = "breaking"
	BreakingReportStatusFailed   BreakingReportStatus = "failed"
)

func NewBreakingReportStatus(value string) (BreakingReportStatus, error) {
	switch status := BreakingReportStatus(strings.TrimSpace(value)); status {
	case BreakingReportStatusPassed, BreakingReportStatusBreaking, BreakingReportStatusFailed:
		return status, nil
	default:
		return "", ErrInvalidBreakingStatus
	}
}

func (status BreakingReportStatus) String() string {
	return string(status)
}

func (status BreakingReportStatus) IsValid() bool {
	_, err := NewBreakingReportStatus(status.String())
	return err == nil
}

type BreakingReport struct {
	ID            BreakingReportID
	ModuleID      ModuleID
	ModuleName    ModuleName
	BaseVersionID ModuleVersionID
	BaseVersion   Version
	TargetRef     string
	Status        BreakingReportStatus
	ChangeCount   int
	RawOutput     string
	HumanSummary  string
	CreatedAt     time.Time
}

type BreakingChange struct {
	ID          BreakingChangeID
	ReportID    BreakingReportID
	Category    string
	FilePath    string
	PackageName string
	Symbol      string
	RuleID      string
	Message     string
	Severity    string
	CreatedAt   time.Time
}
