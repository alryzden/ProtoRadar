package web

import (
	"net/http"
	"strings"
	"testing"

	"github.com/alryzden/ProtoRadar/internal/usecase/uiquery"
)

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

func TestModuleDetailsShowsOwners(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/user-api")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Owners and Maintainers", "team", "platform-team", "owner", "2026-06-04"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestModuleDetailsShowsEmptyOwnersState(t *testing.T) {
	query := newFakeQuery()
	module := query.modules[0]
	module.Owners = nil
	query.modules[0] = module
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/modules/user-api")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "No owners or maintainers configured.") {
		t.Fatalf("body missing owner empty state: %s", res.Body.String())
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

func TestModuleDetailsLinksToRuntimeUsagesPage(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/user-api")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "Runtime Usage") || !strings.Contains(body, `href="/ui/modules/user-api/runtime-usages"`) {
		t.Fatalf("body missing runtime usage link: %s", body)
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
