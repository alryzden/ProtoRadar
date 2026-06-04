package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/uiquery"
)

func TestNewServerParsesTemplates(t *testing.T) {
	if _, err := NewServer(Options{BasePath: "/ui", StaticPath: "/ui/static"}); err != nil {
		t.Fatalf("new server: %v", err)
	}
}

func TestLayoutRendersNavigation(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	body := res.Body.String()
	for _, want := range []string{"ProtoRadar", `href="/ui/modules"`, `href="/ui/breaking-reports"`, "Modules", "Breaking Reports"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestIndexRedirectsToModules(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui")

	if res.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusFound)
	}
	if got := res.Header().Get("Location"); got != "/ui/modules" {
		t.Fatalf("location = %q", got)
	}
}

func TestModulesReturnsModules(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	body := res.Body.String()
	for _, want := range []string{"user-api", "User service", "Repository", "platform/user-api", "v2.0.0", "2", "Reports", "badge badge-breaking"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
	if !strings.Contains(body, `/ui/modules/user-api/versions/v2.0.0`) {
		t.Fatalf("body missing latest version link: %s", body)
	}
	if !strings.Contains(body, `/ui/breaking-reports?module=user-api`) {
		t.Fatalf("body missing filtered reports link: %s", body)
	}
}

func TestModulesAppliesQuery(t *testing.T) {
	query := newFakeQuery()
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/modules?q=billing")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if query.lastListInput.Query != "billing" {
		t.Fatalf("query = %q", query.lastListInput.Query)
	}
	body := res.Body.String()
	if !strings.Contains(body, `value="billing"`) || !strings.Contains(body, "billing-api") {
		t.Fatalf("body missing query/results: %s", body)
	}
	if strings.Contains(body, "user-api") {
		t.Fatalf("body contains unfiltered module: %s", body)
	}
}

func TestModulesEmptyState(t *testing.T) {
	query := newFakeQuery()
	query.modules = nil
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/modules")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "No modules published yet.") || !strings.Contains(body, "protoradar module create") {
		t.Fatalf("body missing empty state: %s", body)
	}
}

func TestModulesSearchEmptyState(t *testing.T) {
	query := newFakeQuery()
	query.modules = nil
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/modules?q=missing")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "No modules match this search.") {
		t.Fatalf("body missing search empty state: %s", body)
	}
}

func TestModuleDetailsRendersHeader(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/user-api")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"user-api", "User service", "https://gitlab.example.com/platform/user-api", "platform/user-api", "v2.0.0", "2026-06-04"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestModuleDetailsRendersVersions(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/user-api")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Published Versions", "v2.0.0", "published", "sha-source-1...cdef", "sha-buf-imag...7890", "passed", "2 files", "1 svc", "3 rpc", "4 msg", "1 enum"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestModuleDetailsRendersRecentReports(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/user-api")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Recent Breaking Reports", "v2.0.0", "feature/remove-field", "breaking", "1", "/ui/breaking-reports/report-2"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestModuleDetailsLinksToDependencyPage(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/user-api")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "Dependency Graph") || !strings.Contains(body, `href="/ui/modules/user-api/dependencies"`) {
		t.Fatalf("body missing dependency graph link: %s", body)
	}
}

func TestModuleDependenciesRendersDownstreamConsumers(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/user-api/dependencies")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Downstream Consumers", "billing-api", "v1.0.0", "import", "import_path", `href="/ui/modules/billing-api"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestModuleDependenciesRendersUpstreamDependencies(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/user-api/dependencies")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Upstream Dependencies", "common-api", "v1.0.0", "type_reference", "symbol", `href="/ui/modules/common-api"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestModuleDependenciesRendersUnresolvedDependencies(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/user-api/dependencies")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Unresolved Dependencies", "missing/v1/missing.proto", "missing.v1.Missing", "provider_not_found"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestModuleDependenciesEmptyStates(t *testing.T) {
	query := newFakeQuery()
	query.dependencyGraphs["user-api"] = uiquery.ModuleDependencyGraph{Module: uiquery.ModuleInfo{Name: "user-api"}}
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/modules/user-api/dependencies")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"No downstream consumers.", "No upstream dependencies.", "No unresolved dependencies."} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestMissingModuleDependencyPageReturnsNotFound(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/missing/dependencies")

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
	if !strings.Contains(res.Body.String(), "Module not found") {
		t.Fatalf("body = %s", res.Body.String())
	}
}

func TestVersionDetailsRendersSummary(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/user-api/versions/v2.0.0")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"user-api", "v2.0.0", "published", "2026-06-04", "Compile Status", "2", "Schema Items"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestVersionDetailsRendersArtifactsAndDigests(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/user-api/versions/v2.0.0")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Stored Artifacts", "source_archive", "buf_image", "sha-source-1...cdef", "sha-buf-imag...7890", "42 bytes", "/api/v1/modules/user-api/versions/v2.0.0/artifact"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestVersionDetailsRendersBufConfigSummary(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/user-api/versions/v2.0.0")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Buf Config Summary", "buf.yaml", "buf.lock", "proto", "buf.build/googleapis", "Lint Status", "Breaking Config"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestVersionDetailsRendersProtoFiles(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/user-api/versions/v2.0.0")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Proto Files", "proto/user/v1/user.proto", "user.v1", "proto3", "google/protobuf/timestamp.proto"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestVersionDetailsRendersServicesMethodsMessagesFieldsEnums(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/user-api/versions/v2.0.0")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Services and Methods", "user.v1.UserService", "GetUser", "user.v1.GetUserRequest", "user.v1.GetUserResponse", "Messages and Fields", "user.v1.User", "display_name", "TYPE_STRING", "Enums and Values", "user.v1.UserStatus", "USER_STATUS_ACTIVE", "1"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestVersionDetailsRendersRelatedReports(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/user-api/versions/v2.0.0")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Related Breaking Reports", "report-2", "feature/remove-field", "breaking", "/ui/breaking-reports/report-2"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestMissingVersionReturnsNotFound(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/user-api/versions/missing")

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
	if !strings.Contains(res.Body.String(), "Module version not found") {
		t.Fatalf("body = %s", res.Body.String())
	}
}

func TestVersionDetailsMissingMetadataRendersEmptyState(t *testing.T) {
	query := newFakeQuery()
	query.version.Metadata = uiquery.DescriptorMetadata{Files: []uiquery.ProtoFile{}}
	query.version.MetadataCounts = uiquery.DescriptorMetadataSummary{}
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/modules/user-api/versions/v2.0.0")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "No descriptor metadata stored for this version.") {
		t.Fatalf("body = %s", res.Body.String())
	}
}

func TestVersionDetailsLongDigestsAreShortenedSafely(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/user-api/versions/v2.0.0")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, `title="sha-source-1234567890abcdef"`) || !strings.Contains(body, "sha-source-1...cdef") {
		t.Fatalf("body missing full/short digest: %s", body)
	}
}

func TestVersionDetailsEscapesProtoMetadataValues(t *testing.T) {
	query := newFakeQuery()
	query.version.Metadata.Files[0].Path = `<script>alert(1)</script>`
	query.version.Metadata.Files[0].Messages[0].Fields[0].Name = `<b>danger</b>`
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/modules/user-api/versions/v2.0.0")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	if strings.Contains(body, `<script>`) || strings.Contains(body, `<b>danger</b>`) {
		t.Fatalf("body rendered raw metadata HTML: %s", body)
	}
	if !strings.Contains(body, `&lt;script&gt;alert(1)&lt;/script&gt;`) || !strings.Contains(body, `&lt;b&gt;danger&lt;/b&gt;`) {
		t.Fatalf("body missing escaped metadata: %s", body)
	}
}

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

func TestMissingModuleReturnsNotFound(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/missing")

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
	if !strings.Contains(res.Body.String(), "Module not found") {
		t.Fatalf("body = %s", res.Body.String())
	}
}

func TestRepositoryAndGitLabURLsAreEscapedSafely(t *testing.T) {
	query := newFakeQuery()
	query.modules[0].Module.RepositoryURL = `https://example.com/?q=<script>`
	query.modules[0].GitLabProject.BaseURL = `https://gitlab.example.com?x=<script>`
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/modules")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	if strings.Contains(body, `<script>`) {
		t.Fatalf("body rendered raw script: %s", body)
	}
	if !strings.Contains(body, `%3cscript%3e`) && !strings.Contains(body, `%3Cscript%3E`) {
		t.Fatalf("body missing escaped URL script: %s", body)
	}
}

func TestModuleNamesAndDescriptionsAreHTMLEscaped(t *testing.T) {
	query := newFakeQuery()
	query.modules = []uiquery.ModuleOverview{{
		Module: uiquery.ModuleInfo{
			Name:          `<script>alert(1)`,
			Description:   `<b>danger</b>`,
			RepositoryURL: "https://example.com/repo",
			CreatedAt:     testTime(1),
			UpdatedAt:     testTime(2),
		},
	}}
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/modules")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	if strings.Contains(body, `<script>`) || strings.Contains(body, `<b>danger</b>`) {
		t.Fatalf("body rendered raw HTML: %s", body)
	}
	if !strings.Contains(body, `&lt;script&gt;alert(1)`) || !strings.Contains(body, `&lt;b&gt;danger&lt;/b&gt;`) {
		t.Fatalf("body missing escaped values: %s", body)
	}
}

func TestStatusBadgeMapping(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{status: "published", want: "badge badge-published"},
		{status: "passed", want: "badge badge-passed"},
		{status: "breaking", want: "badge badge-breaking"},
		{status: "failed", want: "badge badge-failed"},
		{status: "warning", want: "badge badge-warning"},
		{status: "not-a-real-status", want: "badge badge-unknown"},
		{status: "", want: "badge badge-unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := statusBadgeClass(tt.status); got != tt.want {
				t.Fatalf("class = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUnknownStatusRendersNeutralBadge(t *testing.T) {
	query := newFakeQuery()
	query.modules[0].LastBreakingStatus = ""
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/modules")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "badge badge-unknown") {
		t.Fatalf("body missing unknown badge: %s", body)
	}
}

func TestStylesheetReturnsCSS(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/static/app.css")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if contentType := res.Header().Get("Content-Type"); !strings.Contains(contentType, "text/css") {
		t.Fatalf("content type = %q", contentType)
	}
	body := res.Body.String()
	for _, want := range []string{".badge-published", ".badge-breaking", ".table-wrap", ".empty-state", ".error-panel"} {
		if !strings.Contains(body, want) {
			t.Fatalf("css missing %q", want)
		}
	}
}

func TestNotFoundRouteRendersSafeErrorPage(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/does-not-exist")

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
	body := res.Body.String()
	if !strings.Contains(body, "Page Not Found") || !strings.Contains(body, "Back to Modules") {
		t.Fatalf("body missing safe not found content: %s", body)
	}
}

func TestErrorPageDoesNotExposeStackTrace(t *testing.T) {
	server, err := NewServer(Options{BasePath: "/ui", StaticPath: "/ui/static", Query: newFakeQuery()})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	recorder := httptest.NewRecorder()
	server.RenderError(recorder, http.StatusInternalServerError, "panic: secret\ngoroutine 1 [running]\nstack trace")

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	body := recorder.Body.String()
	for _, forbidden := range []string{"panic", "goroutine", "stack trace", "secret"} {
		if strings.Contains(strings.ToLower(body), forbidden) {
			t.Fatalf("error page exposes %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, "Check server logs") {
		t.Fatalf("body missing user-friendly message: %s", body)
	}
}

type fakeQuery struct {
	modules             []uiquery.ModuleOverview
	version             uiquery.VersionOverview
	reports             []uiquery.BreakingReportSummary
	reportDetails       map[string]uiquery.BreakingReportDetails
	dependencyGraphs    map[string]uiquery.ModuleDependencyGraph
	lastListInput       uiquery.ListModuleOverviewsInput
	lastReportListInput uiquery.ListBreakingReportOverviewsInput
}

func newFakeQuery() *fakeQuery {
	return &fakeQuery{
		modules:       []uiquery.ModuleOverview{userModule(), billingModule()},
		version:       userVersionOverview(),
		reports:       []uiquery.BreakingReportSummary{userPassedReport(), userBreakingReport(), billingFailedReport()},
		reportDetails: map[string]uiquery.BreakingReportDetails{"report-2": userBreakingReportDetails()},
		dependencyGraphs: map[string]uiquery.ModuleDependencyGraph{
			"user-api": userDependencyGraph(),
		},
	}
}

func (query *fakeQuery) ListModuleOverviews(ctx context.Context, input uiquery.ListModuleOverviewsInput) ([]uiquery.ModuleOverview, error) {
	query.lastListInput = input
	if strings.TrimSpace(input.Query) == "" {
		return query.modules, nil
	}
	needle := strings.ToLower(strings.TrimSpace(input.Query))
	items := make([]uiquery.ModuleOverview, 0)
	for _, module := range query.modules {
		if strings.Contains(strings.ToLower(module.Module.Name), needle) || strings.Contains(strings.ToLower(module.Module.Description), needle) || strings.Contains(strings.ToLower(module.Module.RepositoryURL), needle) {
			items = append(items, module)
		}
	}
	return items, nil
}

func (query *fakeQuery) GetModuleOverview(ctx context.Context, input uiquery.GetModuleOverviewInput) (uiquery.ModuleOverview, error) {
	for _, module := range query.modules {
		if module.Module.Name == input.Module {
			return module, nil
		}
	}
	return uiquery.ModuleOverview{}, domain.ErrNotFound
}

func (query *fakeQuery) GetVersionOverview(ctx context.Context, input uiquery.GetVersionOverviewInput) (uiquery.VersionOverview, error) {
	if input.Module != "user-api" || input.Version != "v2.0.0" {
		return uiquery.VersionOverview{}, domain.ErrNotFound
	}
	return query.version, nil
}

func (query *fakeQuery) GetModuleDependencyGraph(ctx context.Context, input uiquery.GetModuleDependencyGraphInput) (uiquery.ModuleDependencyGraph, error) {
	graph, ok := query.dependencyGraphs[input.Module]
	if !ok {
		return uiquery.ModuleDependencyGraph{}, domain.ErrNotFound
	}
	return graph, nil
}

func (query *fakeQuery) ListBreakingReportOverviews(ctx context.Context, input uiquery.ListBreakingReportOverviewsInput) ([]uiquery.BreakingReportSummary, error) {
	query.lastReportListInput = input
	items := make([]uiquery.BreakingReportSummary, 0, len(query.reports))
	for _, report := range query.reports {
		if input.Module != "" && report.Module != input.Module {
			continue
		}
		if input.Status != "" && report.Status != input.Status {
			continue
		}
		if input.Query != "" {
			needle := strings.ToLower(strings.TrimSpace(input.Query))
			if !strings.Contains(strings.ToLower(report.ID), needle) &&
				!strings.Contains(strings.ToLower(report.Module), needle) &&
				!strings.Contains(strings.ToLower(report.TargetRef), needle) &&
				!strings.Contains(strings.ToLower(report.BaseVersion), needle) {
				continue
			}
		}
		items = append(items, report)
	}
	return items, nil
}

func (query *fakeQuery) GetBreakingReportDetails(ctx context.Context, input uiquery.GetBreakingReportDetailsInput) (uiquery.BreakingReportDetails, error) {
	details, ok := query.reportDetails[input.ReportID]
	if !ok {
		return uiquery.BreakingReportDetails{}, domain.ErrNotFound
	}
	return details, nil
}

func userModule() uiquery.ModuleOverview {
	latest := uiquery.VersionSummary{Version: "v2.0.0", Status: "published", CreatedAt: testTime(4)}
	return uiquery.ModuleOverview{
		Module: uiquery.ModuleInfo{
			Name:          "user-api",
			Description:   "User service",
			RepositoryURL: "https://gitlab.example.com/platform/user-api",
			CreatedAt:     testTime(1),
			UpdatedAt:     testTime(5),
		},
		GitLabProject:       &uiquery.GitLabProjectInfo{BaseURL: "https://gitlab.example.com", ProjectPath: "platform/user-api", ProjectID: 12345},
		LatestVersion:       &latest,
		VersionCount:        2,
		LastPublishedOrSeen: testTime(7),
		BreakingReportCount: 2,
		LastBreakingStatus:  "breaking",
		Versions: []uiquery.VersionSummary{
			{
				Version:   "v2.0.0",
				Status:    "published",
				CreatedAt: testTime(4),
				Artifacts: []uiquery.ArtifactSummary{
					{Kind: "source_archive", ChecksumSHA256: "sha-source-1234567890abcdef"},
					{Kind: "buf_image", ChecksumSHA256: "sha-buf-image-1234567890"},
				},
				LintStatus:      "passed",
				MetadataSummary: uiquery.DescriptorMetadataSummary{Files: 2, Services: 1, Methods: 3, Messages: 4, Enums: 1},
			},
		},
		RecentReports: []uiquery.BreakingReportSummary{
			userBreakingReport(),
		},
	}
}

func billingModule() uiquery.ModuleOverview {
	latest := uiquery.VersionSummary{Version: "v1.0.0", Status: "published", CreatedAt: testTime(3)}
	return uiquery.ModuleOverview{
		Module:              uiquery.ModuleInfo{Name: "billing-api", Description: "Billing service", RepositoryURL: "https://gitlab.example.com/platform/billing-api", CreatedAt: testTime(2), UpdatedAt: testTime(3)},
		LatestVersion:       &latest,
		VersionCount:        1,
		LastPublishedOrSeen: testTime(3),
	}
}

func userVersionOverview() uiquery.VersionOverview {
	return uiquery.VersionOverview{
		Module: uiquery.ModuleInfo{
			Name:          "user-api",
			Description:   "User service",
			RepositoryURL: "https://gitlab.example.com/platform/user-api",
			CreatedAt:     testTime(1),
			UpdatedAt:     testTime(5),
		},
		Version: uiquery.VersionSummary{
			Version:   "v2.0.0",
			Status:    "published",
			CreatedAt: testTime(4),
			Artifacts: []uiquery.ArtifactSummary{
				{Kind: "source_archive", ChecksumSHA256: "sha-source-1234567890abcdef", SizeBytes: 42},
				{Kind: "buf_image", ChecksumSHA256: "sha-buf-image-1234567890", SizeBytes: 64},
			},
			LintStatus:      "passed",
			MetadataSummary: uiquery.DescriptorMetadataSummary{Files: 2, Packages: 1, Imports: 1, Services: 1, Methods: 1, Messages: 2, Fields: 2, Enums: 1, EnumValues: 2},
		},
		Artifacts: []uiquery.ArtifactSummary{
			{Kind: "source_archive", ChecksumSHA256: "sha-source-1234567890abcdef", SizeBytes: 42},
			{Kind: "buf_image", ChecksumSHA256: "sha-buf-image-1234567890", SizeBytes: 64},
		},
		BufConfig: uiquery.BufConfigSummary{
			ConfigPresent:         true,
			LockPresent:           true,
			ModulePaths:           []string{"proto"},
			Deps:                  []string{"buf.build/googleapis/googleapis"},
			LintEnabled:           true,
			LintStatus:            "passed",
			BreakingConfigPresent: true,
		},
		MetadataCounts: uiquery.DescriptorMetadataSummary{Files: 2, Packages: 1, Imports: 1, Services: 1, Methods: 1, Messages: 2, Fields: 2, Enums: 1, EnumValues: 2},
		Metadata: uiquery.DescriptorMetadata{Files: []uiquery.ProtoFile{{
			Path:        "proto/user/v1/user.proto",
			PackageName: "user.v1",
			Syntax:      "proto3",
			Imports: []uiquery.ProtoImport{
				{Path: "google/protobuf/timestamp.proto", Public: true, Weak: false},
			},
			Services: []uiquery.ProtoService{{
				Name:     "UserService",
				FullName: "user.v1.UserService",
				Methods: []uiquery.ProtoMethod{{
					Name:            "GetUser",
					InputType:       "user.v1.GetUserRequest",
					OutputType:      "user.v1.GetUserResponse",
					ClientStreaming: false,
					ServerStreaming: true,
				}},
			}},
			Messages: []uiquery.ProtoMessage{{
				Name:     "User",
				FullName: "user.v1.User",
				Fields: []uiquery.ProtoField{{
					Name:       "display_name",
					Number:     1,
					Type:       "TYPE_STRING",
					TypeName:   "string",
					Label:      "LABEL_OPTIONAL",
					IsRepeated: false,
					IsMap:      false,
				}},
			}},
			Enums: []uiquery.ProtoEnum{{
				Name:     "UserStatus",
				FullName: "user.v1.UserStatus",
				Values: []uiquery.ProtoEnumValue{
					{Name: "USER_STATUS_UNSPECIFIED", Number: 0},
					{Name: "USER_STATUS_ACTIVE", Number: 1},
				},
			}},
		}}},
		RelatedReports: []uiquery.BreakingReportSummary{
			userBreakingReport(),
		},
	}
}

func userPassedReport() uiquery.BreakingReportSummary {
	return uiquery.BreakingReportSummary{
		ID:          "report-1",
		Module:      "user-api",
		BaseVersion: "v2.0.0",
		TargetRef:   "main",
		Status:      "passed",
		ChangeCount: 0,
		CreatedAt:   testTime(6),
	}
}

func userBreakingReport() uiquery.BreakingReportSummary {
	return uiquery.BreakingReportSummary{
		ID:          "report-2",
		Module:      "user-api",
		BaseVersion: "v2.0.0",
		TargetRef:   "feature/remove-field",
		Status:      "breaking",
		ChangeCount: 1,
		CreatedAt:   testTime(7),
	}
}

func billingFailedReport() uiquery.BreakingReportSummary {
	return uiquery.BreakingReportSummary{
		ID:          "report-3",
		Module:      "billing-api",
		BaseVersion: "v1.0.0",
		TargetRef:   "feature/billing-change",
		Status:      "failed",
		ChangeCount: 0,
		CreatedAt:   testTime(8),
	}
}

func userBreakingReportDetails() uiquery.BreakingReportDetails {
	return uiquery.BreakingReportDetails{
		Report:  userBreakingReport(),
		Summary: "Removed field from user response.\nCoordinate compatibility before merging.",
		Changes: []uiquery.BreakingChangeSummary{{
			FilePath:    "proto/user/v1/user.proto",
			PackageName: "user.v1",
			Symbol:      "user.v1.User.display_name",
			RuleID:      "FIELD_NO_DELETE",
			Message:     "Field was removed",
			Severity:    "breaking",
			CreatedAt:   testTime(7),
		}},
		AffectedModules: []uiquery.DependencyModule{{
			Module:            "billing-api",
			Version:           "v1.0.0",
			DependencySources: []string{"import"},
			Reasons:           []string{"import_path"},
		}},
	}
}

func userDependencyGraph() uiquery.ModuleDependencyGraph {
	return uiquery.ModuleDependencyGraph{
		Module: uiquery.ModuleInfo{Name: "user-api"},
		Downstream: []uiquery.DependencyModule{{
			Module:            "billing-api",
			Version:           "v1.0.0",
			DependencySources: []string{"import"},
			Reasons:           []string{"import_path"},
		}},
		Upstream: []uiquery.DependencyModule{{
			Module:            "common-api",
			Version:           "v1.0.0",
			DependencySources: []string{"type_reference"},
			Reasons:           []string{"symbol"},
		}},
		Unresolved: []uiquery.UnresolvedDependency{{
			Source:           "import",
			ImportPath:       "missing/v1/missing.proto",
			ReferencedSymbol: "missing.v1.Missing",
			Reason:           "provider_not_found",
		}},
	}
}

func newTestHandler(t *testing.T, query Query) http.Handler {
	t.Helper()
	server, err := NewServer(Options{BasePath: "/ui", StaticPath: "/ui/static", Query: query})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return server.Handler()
}

func testTime(hour int) time.Time {
	return time.Date(2026, 6, 4, hour, 0, 0, 0, time.UTC)
}

func request(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

var _ Query = (*fakeQuery)(nil)
