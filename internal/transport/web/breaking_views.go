package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/uiquery"
)

func (server *Server) breakingReports(w http.ResponseWriter, r *http.Request) {
	if server.query == nil {
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	filters := reportFilters{
		Module: strings.TrimSpace(r.URL.Query().Get("module")),
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		Query:  strings.TrimSpace(r.URL.Query().Get("q")),
	}
	filters.HasFilters = filters.Module != "" || filters.Status != "" || filters.Query != ""
	reports, err := server.query.ListBreakingReportOverviews(r.Context(), uiquery.ListBreakingReportOverviewsInput{
		Module: filters.Module,
		Status: filters.Status,
		Query:  filters.Query,
	})
	if err != nil {
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	rows := make([]reportRow, 0, len(reports))
	for _, report := range reports {
		rows = append(rows, server.reportRow(report))
	}

	emptyTitle := "No breaking reports yet."
	emptyBody := "Run a breaking check to create reports."
	if filters.HasFilters {
		emptyTitle = "No breaking reports match these filters."
		emptyBody = "Try a different module, status, or search query."
	}
	server.render(w, http.StatusOK, "breaking_reports.html", pageData{
		Title:         "Breaking Reports",
		Active:        "breaking-reports",
		Status:        "unknown",
		Message:       "Review compatibility checks across modules and target refs.",
		EmptyTitle:    emptyTitle,
		EmptyBody:     emptyBody,
		ReportFilters: filters,
		Reports:       rows,
	})
}

func (server *Server) breakingReportDetail(w http.ResponseWriter, r *http.Request) {
	if server.query == nil {
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	reportID := strings.TrimSpace(r.PathValue("report_id"))
	details, err := server.query.GetBreakingReportDetails(r.Context(), uiquery.GetBreakingReportDetailsInput{ReportID: reportID})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.RenderError(w, http.StatusNotFound, "Breaking report not found.")
			return
		}
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	view := server.breakingReportDetailView(details)
	server.render(w, http.StatusOK, "breaking_report_detail.html", pageData{
		Title:      "Breaking Report " + view.ID,
		Active:     "breaking-reports",
		ReportID:   view.ID,
		Status:     view.Status,
		Message:    "Inspect the summary and individual protobuf compatibility changes.",
		EmptyTitle: "No breaking changes recorded.",
		EmptyBody:  "This report did not record individual breaking-change rows.",
		ReportView: &view,
	})
}
func (server *Server) reportRow(report uiquery.BreakingReportSummary) reportRow {
	row := reportRow{
		ID:          report.ID,
		Module:      report.Module,
		CreatedAt:   formatTime(report.CreatedAt),
		BaseVersion: report.BaseVersion,
		TargetRef:   report.TargetRef,
		Status:      report.Status,
		ChangeCount: report.ChangeCount,
		ReportURL:   server.basePath + "/breaking-reports/" + pathEscape(report.ID),
	}
	if strings.TrimSpace(row.Module) != "" {
		row.ModuleURL = server.basePath + "/modules/" + pathEscape(row.Module)
	}
	if strings.TrimSpace(row.Module) != "" && strings.TrimSpace(row.BaseVersion) != "" {
		row.BaseVersionURL = server.basePath + "/modules/" + pathEscape(row.Module) + "/versions/" + pathEscape(row.BaseVersion)
	}
	return row
}
func (server *Server) breakingReportDetailView(details uiquery.BreakingReportDetails) breakingReportDetailView {
	row := server.reportRow(details.Report)
	view := breakingReportDetailView{
		ID:             row.ID,
		Module:         row.Module,
		BaseVersion:    row.BaseVersion,
		TargetRef:      row.TargetRef,
		Status:         row.Status,
		ChangeCount:    row.ChangeCount,
		CreatedAt:      row.CreatedAt,
		HumanSummary:   details.Summary,
		ModuleURL:      row.ModuleURL,
		BaseVersionURL: row.BaseVersionURL,
		ReportsURL:     server.basePath + "/breaking-reports",
	}
	for _, change := range details.Changes {
		view.Changes = append(view.Changes, changeRow{
			FilePath:    change.FilePath,
			PackageName: change.PackageName,
			Symbol:      change.Symbol,
			RuleID:      change.RuleID,
			Message:     change.Message,
			Severity:    change.Severity,
		})
	}
	for _, item := range details.AffectedModules {
		view.AffectedModules = append(view.AffectedModules, server.dependencyModuleRow(item))
	}
	for _, item := range details.RuntimeImpact {
		view.RuntimeImpact = append(view.RuntimeImpact, server.runtimeImpactRow(item))
	}
	if details.Approval != nil {
		approval := server.approvalRequestView(*details.Approval)
		view.Approval = &approval
		view.HasApproval = true
	}
	view.HasChanges = len(view.Changes) > 0
	view.HasAffectedModules = len(view.AffectedModules) > 0
	view.HasRuntimeImpact = len(view.RuntimeImpact) > 0
	return view
}
