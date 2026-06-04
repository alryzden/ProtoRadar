package gitlabmr

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/alryzden/ProtoRadar/internal/integration/gitlab"
)

const (
	DefaultMaxDisplayedChanges         = 50
	DefaultMaxDisplayedAffectedModules = 20
	statusPassed                       = "passed"
	statusBreaking                     = "breaking"
	statusFailed                       = "failed"
)

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(private[-_ ]?token|access[-_ ]?token|protoradar[-_ ]?token|gitlab[-_ ]?token|authorization|bearer|secret|password)(=|:|\s+)\s*[^\s|]+`),
	regexp.MustCompile(`glpat-[A-Za-z0-9_-]+`),
	regexp.MustCompile(`prr_[A-Za-z0-9_-]+`),
}

type Report struct {
	Module          string
	Against         string
	TargetRef       string
	Status          string
	ChangeCount     int
	Changes         []Change
	AffectedModules []AffectedModule
	ReportID        string
	TargetURL       string
	CommitSHA       string
}

type Change struct {
	FilePath string
	Symbol   string
	RuleID   string
	Message  string
}

type AffectedModule struct {
	Module            string
	LatestVersion     string
	DependencySources []string
	Reasons           []string
}

type RenderOptions struct {
	MaxDisplayedChanges         int
	MaxDisplayedAffectedModules int
}

func BuildMarker(moduleName string) string {
	return "<!-- protoradar:mr-check module=" + markerModuleName(moduleName) + " -->"
}

func ContainsMarker(body string, moduleName string) bool {
	return strings.Contains(body, BuildMarker(moduleName))
}

func SelectExistingNote(notes []gitlab.MergeRequestNote, moduleName string) (gitlab.MergeRequestNote, bool) {
	var selected gitlab.MergeRequestNote
	var matched bool
	var selectedTime time.Time
	var sawTimestamp bool

	for _, note := range notes {
		if !ContainsMarker(note.Body, moduleName) {
			continue
		}

		noteTime := note.UpdatedAt
		if noteTime.IsZero() {
			noteTime = note.CreatedAt
		}
		if !noteTime.IsZero() {
			if !matched || !sawTimestamp || !noteTime.Before(selectedTime) {
				selected = note
				selectedTime = noteTime
				matched = true
				sawTimestamp = true
			}
			continue
		}

		if !sawTimestamp {
			selected = note
			matched = true
		}
	}

	return selected, matched
}

func RenderReport(report Report, options RenderOptions) string {
	maxChanges := options.MaxDisplayedChanges
	if maxChanges <= 0 {
		maxChanges = DefaultMaxDisplayedChanges
	}
	maxAffectedModules := options.MaxDisplayedAffectedModules
	if maxAffectedModules <= 0 {
		maxAffectedModules = DefaultMaxDisplayedAffectedModules
	}

	var builder strings.Builder
	builder.WriteString(BuildMarker(report.Module))
	builder.WriteString("\n")
	builder.WriteString(headerForStatus(report.Status))
	builder.WriteString("\n\n")
	writeSummary(&builder, report)
	writeChanges(&builder, report, maxChanges)
	writeAffectedModules(&builder, report.AffectedModules, maxAffectedModules)
	writeResult(&builder, report.Status)
	writeMetadata(&builder, report)
	return builder.String()
}

func writeSummary(builder *strings.Builder, report Report) {
	builder.WriteString("### Summary\n\n")
	builder.WriteString("| Field | Value |\n")
	builder.WriteString("| --- | --- |\n")
	writeRow(builder, "Module", report.Module)
	writeRow(builder, "Base version / against", report.Against)
	writeRow(builder, "Target commit/ref", report.TargetRef)
	writeRow(builder, "Status", report.Status)
	writeRow(builder, "Change count", fmt.Sprintf("%d", report.ChangeCount))
	builder.WriteString("\n")
}

func writeChanges(builder *strings.Builder, report Report, maxChanges int) {
	builder.WriteString("### Breaking Changes\n\n")
	if reportStatus(report.Status) == statusPassed {
		builder.WriteString("No breaking changes found.\n\n")
		return
	}
	if len(report.Changes) == 0 {
		builder.WriteString("No change details were provided.\n\n")
		return
	}

	limit := len(report.Changes)
	if limit > maxChanges {
		limit = maxChanges
	}
	builder.WriteString("| File | Symbol | Rule | Message |\n")
	builder.WriteString("| --- | --- | --- | --- |\n")
	for _, change := range report.Changes[:limit] {
		writeChangeRow(builder, change)
	}
	builder.WriteString("\n")
	if len(report.Changes) > limit {
		fmt.Fprintf(builder, "_And %d more changes. See the full CI artifact for details._\n\n", len(report.Changes)-limit)
	}
}

func writeAffectedModules(builder *strings.Builder, affected []AffectedModule, maxAffectedModules int) {
	builder.WriteString("### Potentially Affected Modules\n\n")
	if len(affected) == 0 {
		builder.WriteString("No downstream modules are currently known to depend on this module.\n\n")
		return
	}

	limit := len(affected)
	if limit > maxAffectedModules {
		limit = maxAffectedModules
	}
	builder.WriteString("| Module | Latest version | Dependency sources | Reason |\n")
	builder.WriteString("| --- | --- | --- | --- |\n")
	for _, module := range affected[:limit] {
		fmt.Fprintf(builder, "| %s | %s | %s | %s |\n",
			tableCell(defaultIfBlank(module.Module, "unknown module")),
			tableCell(defaultIfBlank(module.LatestVersion, "unknown")),
			tableCell(defaultIfBlank(strings.Join(module.DependencySources, ", "), "unspecified")),
			tableCell(defaultIfBlank(strings.Join(module.Reasons, ", "), "unspecified")),
		)
	}
	builder.WriteString("\n")
	if len(affected) > limit {
		fmt.Fprintf(builder, "_And %d more modules. See ProtoRadar UI for the full dependency graph._\n\n", len(affected)-limit)
	}
}

func writeResult(builder *strings.Builder, status string) {
	builder.WriteString("### Result / Next Steps\n\n")
	switch reportStatus(status) {
	case statusPassed:
		builder.WriteString("Safe to merge from the protobuf compatibility perspective.\n\n")
	case statusBreaking:
		builder.WriteString("Restore compatibility, coordinate a major version, or wait for the future approval workflow.\n\n")
	default:
		builder.WriteString("Check job logs and configuration.\n\n")
	}
}

func writeMetadata(builder *strings.Builder, report Report) {
	if strings.TrimSpace(report.ReportID) == "" && strings.TrimSpace(report.TargetURL) == "" && strings.TrimSpace(report.CommitSHA) == "" {
		return
	}
	builder.WriteString("### Metadata\n\n")
	builder.WriteString("| Field | Value |\n")
	builder.WriteString("| --- | --- |\n")
	if strings.TrimSpace(report.ReportID) != "" {
		writeRow(builder, "Report ID", report.ReportID)
	}
	if strings.TrimSpace(report.TargetURL) != "" {
		writeRow(builder, "Target URL", report.TargetURL)
	}
	if strings.TrimSpace(report.CommitSHA) != "" {
		writeRow(builder, "Commit SHA", report.CommitSHA)
	}
}

func writeRow(builder *strings.Builder, field string, value string) {
	fmt.Fprintf(builder, "| %s | %s |\n", tableCell(field), tableCell(defaultIfBlank(value, "unspecified")))
}

func writeChangeRow(builder *strings.Builder, change Change) {
	fmt.Fprintf(builder, "| %s | %s | %s | %s |\n",
		tableCell(defaultIfBlank(change.FilePath, "unknown file")),
		tableCell(defaultIfBlank(change.Symbol, "unspecified")),
		tableCell(defaultIfBlank(change.RuleID, "unspecified")),
		tableCell(defaultIfBlank(change.Message, "unspecified")),
	)
}

func headerForStatus(status string) string {
	switch reportStatus(status) {
	case statusPassed:
		return "## ✅ ProtoRadar Breaking Change Report"
	case statusBreaking:
		return "## ❌ ProtoRadar Breaking Change Report"
	default:
		return "## ⚠️ ProtoRadar Breaking Change Report"
	}
}

func reportStatus(status string) string {
	value := strings.ToLower(strings.TrimSpace(status))
	if value == "error" || value == statusFailed {
		return statusFailed
	}
	return value
}

func tableCell(value string) string {
	value = redactSecrets(value)
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.Join(strings.Fields(value), " ")
	value = strings.ReplaceAll(value, "|", `\|`)
	const maxCellLength = 500
	if len(value) > maxCellLength {
		value = value[:maxCellLength] + "..."
	}
	return value
}

func redactSecrets(value string) string {
	for _, pattern := range secretPatterns {
		value = pattern.ReplaceAllString(value, "$1[redacted]")
	}
	return value
}

func defaultIfBlank(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func markerModuleName(moduleName string) string {
	value := strings.TrimSpace(moduleName)
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "-->", "")
	return value
}
