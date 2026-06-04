package breaking

import (
	"fmt"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

func FormatReport(report domain.BreakingReport, changes []domain.BreakingChange) string {
	var builder strings.Builder
	writeHeader(&builder, report)

	switch report.Status {
	case domain.BreakingReportStatusPassed:
		builder.WriteString("\nNo breaking changes found.\n")
		builder.WriteString("\nResult: no breaking changes found.\n")
	case domain.BreakingReportStatusBreaking:
		writeChanges(&builder, changes)
		builder.WriteString("\nResult: breaking changes found.\n")
	case domain.BreakingReportStatusFailed:
		if strings.TrimSpace(report.HumanSummary) != "" {
			builder.WriteString("\n")
			builder.WriteString(report.HumanSummary)
			builder.WriteString("\n")
		}
		builder.WriteString("\nResult: breaking check failed.\n")
	default:
		if strings.TrimSpace(report.HumanSummary) != "" {
			builder.WriteString("\n")
			builder.WriteString(report.HumanSummary)
			builder.WriteString("\n")
		}
		builder.WriteString("\nResult: status unknown.\n")
	}

	return builder.String()
}

func writeHeader(builder *strings.Builder, report domain.BreakingReport) {
	builder.WriteString("ProtoRadar Breaking Change Report\n\n")
	if report.ModuleName.String() != "" {
		fmt.Fprintf(builder, "Module: %s\n", report.ModuleName.String())
	}
	if report.BaseVersion.String() != "" {
		fmt.Fprintf(builder, "Against: %s\n", report.BaseVersion.String())
	}
	target := strings.TrimSpace(report.TargetRef)
	if target == "" {
		target = "unspecified"
	}
	fmt.Fprintf(builder, "Target: %s\n", target)
	fmt.Fprintf(builder, "Status: %s\n", report.Status.String())
	fmt.Fprintf(builder, "Changes: %d\n", report.ChangeCount)
}

func writeChanges(builder *strings.Builder, changes []domain.BreakingChange) {
	builder.WriteString("\nBreaking changes:\n")
	if len(changes) == 0 {
		builder.WriteString("No change details were provided.\n")
		return
	}

	for index, change := range changes {
		filePath := strings.TrimSpace(change.FilePath)
		if filePath == "" {
			filePath = "(unknown file)"
		}
		fmt.Fprintf(builder, "%d. %s\n", index+1, filePath)
		writeOptionalLine(builder, "Rule", change.RuleID)
		writeOptionalLine(builder, "Symbol", change.Symbol)
		writeOptionalLine(builder, "Package", change.PackageName)
		writeOptionalLine(builder, "Category", change.Category)
		writeOptionalLine(builder, "Severity", change.Severity)
		writeOptionalLine(builder, "Message", change.Message)
	}
}

func writeOptionalLine(builder *strings.Builder, label string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	fmt.Fprintf(builder, "   %s: %s\n", label, value)
}
