package web

import (
	"net/http"
	"strings"
	"testing"
)

func TestBreakingReportsListRendersReports(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/breaking-reports")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Breaking Check Results", "report-2", "user-api", "v2.0.0", "feature/remove-field", "breaking", "Report details", "/ui/modules/user-api", "/ui/modules/user-api/versions/v2.0.0"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestBreakingReportsListFiltersByModule(t *testing.T) {
	query := newFakeQuery()
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/breaking-reports?module=billing-api")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if query.lastReportListInput.Module != "billing-api" {
		t.Fatalf("module filter = %q", query.lastReportListInput.Module)
	}
	body := res.Body.String()
	if !strings.Contains(body, "billing-api") || strings.Contains(body, "report-2") || strings.Contains(body, "feature/remove-field") {
		t.Fatalf("body has wrong filtered reports: %s", body)
	}
}

func TestBreakingReportsListFiltersByStatus(t *testing.T) {
	query := newFakeQuery()
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/breaking-reports?status=passed")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if query.lastReportListInput.Status != "passed" {
		t.Fatalf("status filter = %q", query.lastReportListInput.Status)
	}
	body := res.Body.String()
	if !strings.Contains(body, "report-1") || !strings.Contains(body, "passed") || strings.Contains(body, "feature/remove-field") {
		t.Fatalf("body has wrong status-filtered reports: %s", body)
	}
}

func TestBreakingReportsListEmptyState(t *testing.T) {
	query := newFakeQuery()
	query.reports = nil
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/breaking-reports")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "No breaking reports yet.") {
		t.Fatalf("body missing empty state: %s", res.Body.String())
	}
}

func TestBreakingReportsListFilteredEmptyState(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/breaking-reports?module=missing")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "No breaking reports match these filters.") {
		t.Fatalf("body missing filtered empty state: %s", res.Body.String())
	}
}

func TestBreakingReportDetailsRendersSummary(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/breaking-reports/report-2")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"report-2", "user-api", "v2.0.0", "feature/remove-field", "breaking", "1", "Removed field from user response", "2026-06-04"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestBreakingReportDetailsRendersHumanSummarySafely(t *testing.T) {
	query := newFakeQuery()
	details := query.reportDetails["report-2"]
	details.Summary = `<script>alert(1)</script>
Check generated code.`
	query.reportDetails["report-2"] = details
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/breaking-reports/report-2")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	if strings.Contains(body, `<script>`) {
		t.Fatalf("body rendered raw summary HTML: %s", body)
	}
	if !strings.Contains(body, `&lt;script&gt;alert(1)&lt;/script&gt;`) {
		t.Fatalf("body missing escaped summary: %s", body)
	}
}

func TestBreakingReportDetailsRendersChangesTable(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/breaking-reports/report-2")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Detected Changes", "proto/user/v1/user.proto", "user.v1", "user.v1.User.display_name", "FIELD_NO_DELETE", "Field was removed", "breaking"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestBreakingReportDetailsLinksToModuleAndBaseVersion(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/breaking-reports/report-2")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{`href="/ui/modules/user-api"`, `href="/ui/modules/user-api/versions/v2.0.0"`, `href="/ui/breaking-reports"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestMissingBreakingReportReturnsNotFound(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/breaking-reports/missing")

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
	if !strings.Contains(res.Body.String(), "Breaking report not found") {
		t.Fatalf("body = %s", res.Body.String())
	}
}

func TestBreakingReportMessagesAreHTMLEscaped(t *testing.T) {
	query := newFakeQuery()
	details := query.reportDetails["report-2"]
	details.Changes[0].Message = `<b>danger</b>`
	query.reportDetails["report-2"] = details
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/breaking-reports/report-2")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	if strings.Contains(body, `<b>danger</b>`) {
		t.Fatalf("body rendered raw change HTML: %s", body)
	}
	if !strings.Contains(body, `&lt;b&gt;danger&lt;/b&gt;`) {
		t.Fatalf("body missing escaped change message: %s", body)
	}
}

func TestBreakingReportStatusBadgesRenderClassesAndLabels(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/breaking-reports")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"badge badge-passed", "badge badge-breaking", ">passed<", ">breaking<"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestBreakingReportDetailsShowsAffectedModules(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/breaking-reports/report-2")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Potentially Affected Modules", "billing-api", "v1.0.0", "import", "import_path"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestBreakingReportDetailsShowsApprovalStatus(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/breaking-reports/report-2")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Approval Status", "approval-1", "pending", "module_owner_approval", "No owners are configured", `href="/ui/approval-requests/approval-1"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestBreakingReportDetailsAffectedModulesEmptyState(t *testing.T) {
	query := newFakeQuery()
	details := query.reportDetails["report-2"]
	details.AffectedModules = nil
	query.reportDetails["report-2"] = details
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/breaking-reports/report-2")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "No downstream modules are currently known to depend on this module.") {
		t.Fatalf("body missing affected modules empty state: %s", res.Body.String())
	}
}

func TestBreakingReportDetailsRendersRuntimeImpactSection(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/breaking-reports/report-2")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Runtime Impact", "billing-service", "production", "v2.0.0", "2026.06.04-15", "abc1234", "potentially_affected_by_breaking_change", "service uses base module version"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestBreakingReportDetailsRendersDeprecatedRuntimeImpactDrift(t *testing.T) {
	query := newFakeQuery()
	details := query.reportDetails["report-2"]
	details.RuntimeImpact[0].DriftStatus = "deprecated_version"
	details.RuntimeImpact[0].DriftReason = `Use v2.1.0 <now>`
	query.reportDetails["report-2"] = details
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/breaking-reports/report-2")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Runtime Drift", "deprecated_version", "Use v2.1.0 &lt;now&gt;"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
	if strings.Contains(body, `Use v2.1.0 <now>`) {
		t.Fatalf("body rendered raw drift reason: %s", body)
	}
}

func TestBreakingReportDetailsRuntimeImpactEmptyState(t *testing.T) {
	query := newFakeQuery()
	details := query.reportDetails["report-2"]
	details.RuntimeImpact = nil
	query.reportDetails["report-2"] = details
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/breaking-reports/report-2")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "No runtime services are currently known to use the affected module version.") {
		t.Fatalf("body missing runtime impact empty state: %s", res.Body.String())
	}
}
