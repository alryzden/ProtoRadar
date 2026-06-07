package httptransport

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/alryzden/ProtoRadar/internal/authorization"
	"github.com/alryzden/ProtoRadar/internal/domain"
)

func TestRouteClassificationCoverageHasNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, tt := range routeClassificationExpectations() {
		key := tt.method + " " + tt.path
		if seen[key] {
			t.Fatalf("duplicate route coverage entry %q", key)
		}
		seen[key] = true
	}
}

func TestRouteClassificationCoverageMatchesServerRouteRegistration(t *testing.T) {
	source, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("read server.go: %v", err)
	}
	classified := map[string]routeClassificationExpectation{}
	for _, tt := range routeClassificationExpectations() {
		classified[tt.method+" "+tt.path] = tt
	}
	registered := map[string]bool{}

	for _, line := range strings.Split(string(source), "\n") {
		if !strings.Contains(line, `mux.Handle("`) && !strings.Contains(line, `mux.HandleFunc("`) {
			continue
		}
		route, ok := quotedRoutePattern(line)
		if !ok {
			continue
		}
		expected, ok := classified[route]
		if !ok {
			t.Fatalf("registered route %q is missing from route classification table", route)
		}
		registered[route] = true
		assertRouteRegistrationMatchesClassification(t, line, expected)
	}
	for route := range classified {
		if !registered[route] {
			t.Fatalf("classified route %q is not registered in server.go", route)
		}
	}
}

func TestRouteClassificationsCoverExpectedPublicBootstrapAndProtectedRoutes(t *testing.T) {
	classified := map[string]routeClassificationExpectation{}
	for _, tt := range routeClassificationExpectations() {
		classified[tt.method+" "+tt.path] = tt
	}

	for _, route := range []string{"GET /healthz", "GET /readyz", "GET /metrics"} {
		assertRouteClassified(t, classified, route, routeClassificationPublic)
	}
	assertRouteClassified(t, classified, "POST /api/v1/tokens", routeClassificationBootstrap)
	for _, tt := range authenticatedRouteExpectations() {
		route := tt.method + " " + templatePath(tt.path)
		classification := assertRouteClassified(t, classified, route, routeClassificationProtected)
		if classification.action != tt.action {
			t.Fatalf("%s action = %s, want %s", route, classification.action, tt.action)
		}
		if classification.resourceType != tt.resource.Type {
			t.Fatalf("%s resource type = %q, want %q", route, classification.resourceType, tt.resource.Type)
		}
	}
}

func assertRouteRegistrationMatchesClassification(t *testing.T, line string, expected routeClassificationExpectation) {
	t.Helper()
	trimmed := strings.TrimSpace(line)
	switch expected.classification {
	case routeClassificationPublic:
		for _, forbidden := range []string{"server.protected(", "requireBearer", "requireBootstrapToken"} {
			if strings.Contains(line, forbidden) {
				t.Fatalf("public route uses %s: %s", forbidden, trimmed)
			}
		}
	case routeClassificationBootstrap:
		if !strings.Contains(line, "requireBootstrapToken") {
			t.Fatalf("bootstrap route is not using bootstrap-token guard: %s", trimmed)
		}
		if strings.Contains(line, "server.protected(") || strings.Contains(line, "requireBearer") {
			t.Fatalf("bootstrap route uses normal authorization path: %s", trimmed)
		}
	case routeClassificationProtected:
		if !strings.Contains(line, "server.protected(") {
			t.Fatalf("protected route is not protected: %s", trimmed)
		}
		if strings.Contains(line, "requireBearer") {
			t.Fatalf("protected route bypasses Authorizer with direct requireBearer: %s", trimmed)
		}
		if !strings.Contains(line, string(expected.action)) && !strings.Contains(line, actionConstantName(expected.action)) {
			t.Fatalf("protected route %s does not show expected action %s in registration: %s", expected.path, expected.action, trimmed)
		}
	default:
		t.Fatalf("unknown classification %q for route %s %s", expected.classification, expected.method, expected.path)
	}
}

func assertRouteClassified(t *testing.T, classified map[string]routeClassificationExpectation, route string, want routeClassification) routeClassificationExpectation {
	t.Helper()
	classification, ok := classified[route]
	if !ok {
		t.Fatalf("route %q is missing from route classification table", route)
	}
	if classification.classification != want {
		t.Fatalf("route %q classification = %q, want %q", route, classification.classification, want)
	}
	if !classification.covered {
		t.Fatalf("route %q is not marked as covered", route)
	}
	return classification
}

func actionConstantName(action authorization.Action) string {
	switch action {
	case authorization.ActionModuleCreate:
		return "ActionModuleCreate"
	case authorization.ActionModuleRead:
		return "ActionModuleRead"
	case authorization.ActionModuleVersionPublish:
		return "ActionModuleVersionPublish"
	case authorization.ActionModuleVersionRead:
		return "ActionModuleVersionRead"
	case authorization.ActionModuleVersionDownload:
		return "ActionModuleVersionDownload"
	case authorization.ActionModuleVersionDeprecate:
		return "ActionModuleVersionDeprecate"
	case authorization.ActionGitLabMappingManage:
		return "ActionGitLabMappingManage"
	case authorization.ActionGitLabMappingRead:
		return "ActionGitLabMappingRead"
	case authorization.ActionBreakingCheckRun:
		return "ActionBreakingCheckRun"
	case authorization.ActionBreakingReportRead:
		return "ActionBreakingReportRead"
	case authorization.ActionDependencyGraphRead:
		return "ActionDependencyGraphRead"
	case authorization.ActionRuntimeInventoryReport:
		return "ActionRuntimeInventoryReport"
	case authorization.ActionRuntimeInventoryRead:
		return "ActionRuntimeInventoryRead"
	case authorization.ActionGovernanceOwnerManage:
		return "ActionGovernanceOwnerManage"
	case authorization.ActionGovernanceOwnerRead:
		return "ActionGovernanceOwnerRead"
	case authorization.ActionApprovalRequestCreate:
		return "ActionApprovalRequestCreate"
	case authorization.ActionApprovalRequestRead:
		return "ActionApprovalRequestRead"
	case authorization.ActionApprovalDecisionRecord:
		return "ActionApprovalDecisionRecord"
	case authorization.ActionGovernanceAuditRead:
		return "ActionGovernanceAuditRead"
	case authorization.ActionEditionRead:
		return "ActionEditionRead"
	default:
		return ""
	}
}

func quotedRoutePattern(line string) (string, bool) {
	start := strings.Index(line, `"`)
	if start < 0 {
		return "", false
	}
	rest := line[start+1:]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return "", false
	}
	return rest[:end], true
}

func templatePath(path string) string {
	replacer := strings.NewReplacer(
		"user-api", "{module}",
		"v1.0.0", "{version}",
		"report-1", "{report_id}",
		"owner-1", "{owner_id}",
		"request-1", "{request_id}",
		"requirement-1", "{requirement_id}",
		"billing-service", "{service}",
		"production", "{environment}",
	)
	return replacer.Replace(path)
}

type routeClassification string

const (
	routeClassificationPublic    routeClassification = "public"
	routeClassificationBootstrap routeClassification = "bootstrap"
	routeClassificationProtected routeClassification = "protected"
)

type routeClassificationExpectation struct {
	method         string
	path           string
	classification routeClassification
	action         authorization.Action
	resourceType   string
	covered        bool
}

func routeClassificationExpectations() []routeClassificationExpectation {
	routes := []routeClassificationExpectation{
		{method: http.MethodGet, path: "/healthz", classification: routeClassificationPublic, covered: true},
		{method: http.MethodGet, path: "/readyz", classification: routeClassificationPublic, covered: true},
		{method: http.MethodGet, path: "/metrics", classification: routeClassificationPublic, covered: true},
		{method: http.MethodPost, path: "/api/v1/tokens", classification: routeClassificationBootstrap, covered: true},
	}
	for _, tt := range authenticatedRouteExpectations() {
		routes = append(routes, routeClassificationExpectation{
			method:         tt.method,
			path:           templatePath(tt.path),
			classification: routeClassificationProtected,
			action:         tt.action,
			resourceType:   tt.resource.Type,
			covered:        true,
		})
	}
	return routes
}

func coreRegistryAuthorizationRoutes() []protectedRouteAuthorizationCase {
	jsonBody := func(body string) func() (io.Reader, string) {
		return func() (io.Reader, string) {
			return strings.NewReader(body), "application/json"
		}
	}
	noBody := func() (io.Reader, string) {
		return nil, ""
	}
	multipartArtifactBody := func(fields map[string]string) func() (io.Reader, string) {
		return func() (io.Reader, string) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			for name, value := range fields {
				if err := writer.WriteField(name, value); err != nil {
					panic(err)
				}
			}
			file, err := writer.CreateFormFile("artifact", "artifact.tar.gz")
			if err != nil {
				panic(err)
			}
			if _, err := file.Write([]byte("artifact")); err != nil {
				panic(err)
			}
			if err := writer.Close(); err != nil {
				panic(err)
			}
			return &body, writer.FormDataContentType()
		}
	}

	return []protectedRouteAuthorizationCase{
		{
			name:          "create module",
			method:        http.MethodPost,
			path:          "/api/v1/modules",
			requestBody:   jsonBody(`{"name":"user-api"}`),
			action:        authorization.ActionModuleCreate,
			resource:      authorization.Resource{Type: "module_collection"},
			allowedStatus: http.StatusCreated,
		},
		{
			name:          "list modules",
			method:        http.MethodGet,
			path:          "/api/v1/modules",
			requestBody:   noBody,
			action:        authorization.ActionModuleRead,
			resource:      authorization.Resource{Type: "module_collection"},
			allowedStatus: http.StatusOK,
		},
		// The focused harness starts with an empty fake registry, so reads for a
		// specific module prove the handler was reached by returning not_found.
		{
			name:          "get module",
			method:        http.MethodGet,
			path:          "/api/v1/modules/user-api",
			requestBody:   noBody,
			action:        authorization.ActionModuleRead,
			resource:      authorization.Resource{Type: "module", Name: "user-api"},
			allowedStatus: http.StatusNotFound,
		},
		{
			name:          "publish module version",
			method:        http.MethodPost,
			path:          "/api/v1/modules/user-api/versions",
			requestBody:   multipartArtifactBody(map[string]string{"version": "v1.0.0"}),
			action:        authorization.ActionModuleVersionPublish,
			resource:      authorization.Resource{Type: "module", Name: "user-api"},
			allowedStatus: http.StatusNotFound,
		},
		{
			name:          "list module versions",
			method:        http.MethodGet,
			path:          "/api/v1/modules/user-api/versions",
			requestBody:   noBody,
			action:        authorization.ActionModuleVersionRead,
			resource:      authorization.Resource{Type: "module", Name: "user-api"},
			allowedStatus: http.StatusNotFound,
		},
		{
			name:          "get module version",
			method:        http.MethodGet,
			path:          "/api/v1/modules/user-api/versions/v1.0.0",
			requestBody:   noBody,
			action:        authorization.ActionModuleVersionRead,
			resource:      authorization.ModuleVersionResource("user-api", "v1.0.0"),
			allowedStatus: http.StatusNotFound,
		},
		{
			name:          "deprecate module version",
			method:        http.MethodPost,
			path:          "/api/v1/modules/user-api/versions/v1.0.0/deprecate",
			requestBody:   jsonBody(`{"reason":"Use v1.1.0 instead."}`),
			action:        authorization.ActionModuleVersionDeprecate,
			resource:      authorization.ModuleVersionResource("user-api", "v1.0.0"),
			allowedStatus: http.StatusNotFound,
		},
		{
			name:          "get module version metadata",
			method:        http.MethodGet,
			path:          "/api/v1/modules/user-api/versions/v1.0.0/metadata",
			requestBody:   noBody,
			action:        authorization.ActionModuleVersionRead,
			resource:      authorization.ModuleVersionResource("user-api", "v1.0.0"),
			allowedStatus: http.StatusNotFound,
		},
		{
			name:          "download module version artifact",
			method:        http.MethodGet,
			path:          "/api/v1/modules/user-api/versions/v1.0.0/artifact",
			requestBody:   noBody,
			action:        authorization.ActionModuleVersionDownload,
			resource:      authorization.ModuleVersionResource("user-api", "v1.0.0"),
			allowedStatus: http.StatusNotFound,
		},
		{
			name:          "run breaking check",
			method:        http.MethodPost,
			path:          "/api/v1/modules/user-api/breaking-checks",
			requestBody:   multipartArtifactBody(map[string]string{"against": "latest", "target_ref": "main"}),
			action:        authorization.ActionBreakingCheckRun,
			resource:      authorization.Resource{Type: "module", Name: "user-api"},
			allowedStatus: http.StatusNotFound,
		},
		{
			name:          "list breaking reports",
			method:        http.MethodGet,
			path:          "/api/v1/modules/user-api/breaking-reports",
			requestBody:   noBody,
			action:        authorization.ActionBreakingReportRead,
			resource:      authorization.Resource{Type: "module", Name: "user-api"},
			allowedStatus: http.StatusNotFound,
		},
		{
			name:          "get breaking report",
			method:        http.MethodGet,
			path:          "/api/v1/breaking-reports/report-1",
			requestBody:   noBody,
			action:        authorization.ActionBreakingReportRead,
			resource:      authorization.Resource{Type: "breaking_report", ID: "report-1"},
			allowedStatus: http.StatusNotFound,
		},
	}
}

func integrationRuntimeAndEditionAuthorizationRoutes() []protectedRouteAuthorizationCase {
	jsonBody := func(body string) func() (io.Reader, string) {
		return func() (io.Reader, string) {
			return strings.NewReader(body), "application/json"
		}
	}
	noBody := func() (io.Reader, string) {
		return nil, ""
	}

	return []protectedRouteAuthorizationCase{
		{
			name:   "manage gitlab mapping",
			method: http.MethodPut,
			path:   "/api/v1/modules/user-api/gitlab-project",
			requestBody: jsonBody(`{
				"gitlab_base_url": "https://gitlab.example.com",
				"gitlab_project_id": 123,
				"gitlab_project_path": "platform/user-api"
			}`),
			action:        authorization.ActionGitLabMappingManage,
			resource:      authorization.Resource{Type: "module", Name: "user-api"},
			allowedStatus: http.StatusNotFound,
		},
		{
			name:          "read gitlab mapping",
			method:        http.MethodGet,
			path:          "/api/v1/modules/user-api/gitlab-project",
			requestBody:   noBody,
			action:        authorization.ActionGitLabMappingRead,
			resource:      authorization.Resource{Type: "module", Name: "user-api"},
			allowedStatus: http.StatusNotFound,
		},
		{
			name:          "read module dependency graph",
			method:        http.MethodGet,
			path:          "/api/v1/modules/user-api/dependencies",
			requestBody:   noBody,
			action:        authorization.ActionDependencyGraphRead,
			resource:      authorization.Resource{Type: "module", Name: "user-api"},
			allowedStatus: http.StatusNotFound,
		},
		{
			name:          "read affected modules",
			method:        http.MethodGet,
			path:          "/api/v1/modules/user-api/affected",
			requestBody:   noBody,
			action:        authorization.ActionDependencyGraphRead,
			resource:      authorization.Resource{Type: "module", Name: "user-api"},
			allowedStatus: http.StatusNotFound,
		},
		{
			name:          "read breaking report affected modules",
			method:        http.MethodGet,
			path:          "/api/v1/breaking-reports/report-1/affected-modules",
			requestBody:   noBody,
			action:        authorization.ActionBreakingReportRead,
			resource:      authorization.Resource{Type: "breaking_report", ID: "report-1"},
			allowedStatus: http.StatusNotFound,
		},
		{
			name:   "report runtime inventory",
			method: http.MethodPost,
			path:   "/api/v1/runtime/reports",
			requestBody: jsonBody(`{
				"service_name": "billing-service",
				"environment": "production",
				"git_commit": "abc123",
				"build_version": "v1",
				"modules": [{"module": "user-api", "version": "v1.0.0"}]
			}`),
			action:        authorization.ActionRuntimeInventoryReport,
			resource:      authorization.Resource{Type: "runtime_inventory"},
			allowedStatus: http.StatusCreated,
		},
		{
			name:          "list runtime services",
			method:        http.MethodGet,
			path:          "/api/v1/runtime/services",
			requestBody:   noBody,
			action:        authorization.ActionRuntimeInventoryRead,
			resource:      authorization.Resource{Type: "runtime_inventory"},
			allowedStatus: http.StatusOK,
		},
		{
			name:          "read runtime service details",
			method:        http.MethodGet,
			path:          "/api/v1/runtime/services/billing-service",
			requestBody:   noBody,
			action:        authorization.ActionRuntimeInventoryRead,
			resource:      authorization.Resource{Type: "runtime_service", Name: "billing-service"},
			allowedStatus: http.StatusNotFound,
		},
		{
			name:          "read runtime environment inventory",
			method:        http.MethodGet,
			path:          "/api/v1/runtime/environments/production",
			requestBody:   noBody,
			action:        authorization.ActionRuntimeInventoryRead,
			resource:      authorization.Resource{Type: "runtime_environment", Name: "production"},
			allowedStatus: http.StatusOK,
		},
		{
			name:          "read module runtime usages",
			method:        http.MethodGet,
			path:          "/api/v1/modules/user-api/runtime-usages",
			requestBody:   noBody,
			action:        authorization.ActionRuntimeInventoryRead,
			resource:      authorization.Resource{Type: "module", Name: "user-api"},
			allowedStatus: http.StatusOK,
		},
		{
			name:          "read breaking report runtime impact",
			method:        http.MethodGet,
			path:          "/api/v1/breaking-reports/report-1/runtime-impact",
			requestBody:   noBody,
			action:        authorization.ActionBreakingReportRead,
			resource:      authorization.Resource{Type: "breaking_report", ID: "report-1"},
			allowedStatus: http.StatusNotFound,
		},
		{
			name:          "read edition",
			method:        http.MethodGet,
			path:          "/api/v1/edition",
			requestBody:   noBody,
			action:        authorization.ActionEditionRead,
			resource:      authorization.Resource{Type: "edition"},
			allowedStatus: http.StatusOK,
		},
	}
}

func governanceAuthorizationRoutes() []protectedRouteAuthorizationCase {
	jsonBody := func(body string) func() (io.Reader, string) {
		return func() (io.Reader, string) {
			return strings.NewReader(body), "application/json"
		}
	}
	noBody := func() (io.Reader, string) {
		return nil, ""
	}

	return []protectedRouteAuthorizationCase{
		{
			name:          "list module owners",
			method:        http.MethodGet,
			path:          "/api/v1/modules/user-api/owners",
			requestBody:   noBody,
			action:        authorization.ActionGovernanceOwnerRead,
			resource:      authorization.Resource{Type: "module", Name: "user-api"},
			allowedStatus: http.StatusOK,
		},
		{
			name:   "add module owner",
			method: http.MethodPost,
			path:   "/api/v1/modules/user-api/owners",
			requestBody: jsonBody(`{
				"subject_type": "user",
				"subject": "alice",
				"role": "owner"
			}`),
			action:        authorization.ActionGovernanceOwnerManage,
			resource:      authorization.Resource{Type: "module", Name: "user-api"},
			allowedStatus: http.StatusCreated,
		},
		{
			name:          "remove module owner",
			method:        http.MethodDelete,
			path:          "/api/v1/modules/user-api/owners/owner-1",
			requestBody:   noBody,
			action:        authorization.ActionGovernanceOwnerManage,
			resource:      authorization.Resource{Type: "module_owner", ID: "owner-1"},
			allowedStatus: http.StatusNoContent,
		},
		{
			name:          "create approval request",
			method:        http.MethodPost,
			path:          "/api/v1/breaking-reports/report-1/approval-request",
			requestBody:   jsonBody(`{}`),
			action:        authorization.ActionApprovalRequestCreate,
			resource:      authorization.Resource{Type: "breaking_report", ID: "report-1"},
			allowedStatus: http.StatusCreated,
		},
		{
			name:          "read approval status",
			method:        http.MethodGet,
			path:          "/api/v1/breaking-reports/report-1/approval-status",
			requestBody:   noBody,
			action:        authorization.ActionApprovalRequestRead,
			resource:      authorization.Resource{Type: "breaking_report", ID: "report-1"},
			allowedStatus: http.StatusOK,
		},
		{
			name:          "approve requirement",
			method:        http.MethodPost,
			path:          "/api/v1/approval-requests/request-1/requirements/requirement-1/approve",
			requestBody:   jsonBody(`{"comment":"ok"}`),
			action:        authorization.ActionApprovalDecisionRecord,
			resource:      authorization.Resource{Type: "approval_request", ID: "request-1"},
			allowedStatus: http.StatusOK,
		},
		{
			name:          "reject requirement",
			method:        http.MethodPost,
			path:          "/api/v1/approval-requests/request-1/requirements/requirement-1/reject",
			requestBody:   jsonBody(`{"comment":"no"}`),
			action:        authorization.ActionApprovalDecisionRecord,
			resource:      authorization.Resource{Type: "approval_request", ID: "request-1"},
			allowedStatus: http.StatusOK,
		},
		{
			name:          "read approval audit",
			method:        http.MethodGet,
			path:          "/api/v1/approval-requests/request-1/audit",
			requestBody:   noBody,
			action:        authorization.ActionGovernanceAuditRead,
			resource:      authorization.Resource{Type: "approval_request", ID: "request-1"},
			allowedStatus: http.StatusOK,
		},
	}
}

type authenticatedRouteExpectation struct {
	method      string
	path        string
	body        func() io.Reader
	contentType string
	action      authorization.Action
	resource    authorization.Resource
}

func (route authenticatedRouteExpectation) bodyReader() io.Reader {
	if route.body == nil {
		return nil
	}
	return route.body()
}

func authenticatedRouteExpectations() []authenticatedRouteExpectation {
	jsonBody := func(body string) func() io.Reader {
		return func() io.Reader {
			return strings.NewReader(body)
		}
	}
	multipartBody := func(fields map[string]string) (func() io.Reader, string) {
		var contentType string
		return func() io.Reader {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			for name, value := range fields {
				if err := writer.WriteField(name, value); err != nil {
					panic(err)
				}
			}
			file, err := writer.CreateFormFile("artifact", "artifact.tar.gz")
			if err != nil {
				panic(err)
			}
			if _, err := file.Write([]byte("artifact")); err != nil {
				panic(err)
			}
			if err := writer.Close(); err != nil {
				panic(err)
			}
			contentType = writer.FormDataContentType()
			return &body
		}, contentType
	}
	versionBody, versionContentType := multipartBody(map[string]string{"version": "v1.0.0"})
	breakingBody, breakingContentType := multipartBody(map[string]string{"against": "latest", "target_ref": "main"})

	return []authenticatedRouteExpectation{
		{method: http.MethodGet, path: "/api/v1/edition", action: authorization.ActionEditionRead, resource: authorization.Resource{Type: "edition"}},
		{method: http.MethodPost, path: "/api/v1/modules", body: jsonBody(`{"name":"user-api"}`), contentType: "application/json", action: authorization.ActionModuleCreate, resource: authorization.Resource{Type: "module_collection"}},
		{method: http.MethodGet, path: "/api/v1/modules", action: authorization.ActionModuleRead, resource: authorization.Resource{Type: "module_collection"}},
		{method: http.MethodGet, path: "/api/v1/modules/user-api", action: authorization.ActionModuleRead, resource: authorization.Resource{Type: "module", Name: "user-api"}},
		{method: http.MethodPut, path: "/api/v1/modules/user-api/gitlab-project", body: jsonBody(`{"gitlab_base_url":"https://gitlab.example.com","gitlab_project_id":123,"gitlab_project_path":"platform/user-api"}`), contentType: "application/json", action: authorization.ActionGitLabMappingManage, resource: authorization.Resource{Type: "module", Name: "user-api"}},
		{method: http.MethodGet, path: "/api/v1/modules/user-api/gitlab-project", action: authorization.ActionGitLabMappingRead, resource: authorization.Resource{Type: "module", Name: "user-api"}},
		{method: http.MethodGet, path: "/api/v1/modules/user-api/dependencies", action: authorization.ActionDependencyGraphRead, resource: authorization.Resource{Type: "module", Name: "user-api"}},
		{method: http.MethodGet, path: "/api/v1/modules/user-api/affected", action: authorization.ActionDependencyGraphRead, resource: authorization.Resource{Type: "module", Name: "user-api"}},
		{method: http.MethodGet, path: "/api/v1/modules/user-api/owners", action: authorization.ActionGovernanceOwnerRead, resource: authorization.Resource{Type: "module", Name: "user-api"}},
		{method: http.MethodPost, path: "/api/v1/modules/user-api/owners", body: jsonBody(`{"subject_type":"user","subject":"alice","role":"owner"}`), contentType: "application/json", action: authorization.ActionGovernanceOwnerManage, resource: authorization.Resource{Type: "module", Name: "user-api"}},
		{method: http.MethodDelete, path: "/api/v1/modules/user-api/owners/owner-1", action: authorization.ActionGovernanceOwnerManage, resource: authorization.Resource{Type: "module_owner", ID: "owner-1"}},
		{method: http.MethodGet, path: "/api/v1/modules/user-api/runtime-usages", action: authorization.ActionRuntimeInventoryRead, resource: authorization.Resource{Type: "module", Name: "user-api"}},
		{method: http.MethodPost, path: "/api/v1/modules/user-api/versions", body: versionBody, contentType: versionContentType, action: authorization.ActionModuleVersionPublish, resource: authorization.Resource{Type: "module", Name: "user-api"}},
		{method: http.MethodGet, path: "/api/v1/modules/user-api/versions", action: authorization.ActionModuleVersionRead, resource: authorization.Resource{Type: "module", Name: "user-api"}},
		{method: http.MethodGet, path: "/api/v1/modules/user-api/versions/v1.0.0", action: authorization.ActionModuleVersionRead, resource: authorization.ModuleVersionResource("user-api", "v1.0.0")},
		{method: http.MethodPost, path: "/api/v1/modules/user-api/versions/v1.0.0/deprecate", body: jsonBody(`{"reason":"Use v1.1.0 instead."}`), contentType: "application/json", action: authorization.ActionModuleVersionDeprecate, resource: authorization.ModuleVersionResource("user-api", "v1.0.0")},
		{method: http.MethodGet, path: "/api/v1/modules/user-api/versions/v1.0.0/metadata", action: authorization.ActionModuleVersionRead, resource: authorization.ModuleVersionResource("user-api", "v1.0.0")},
		{method: http.MethodGet, path: "/api/v1/modules/user-api/versions/v1.0.0/artifact", action: authorization.ActionModuleVersionDownload, resource: authorization.ModuleVersionResource("user-api", "v1.0.0")},
		{method: http.MethodPost, path: "/api/v1/modules/user-api/breaking-checks", body: breakingBody, contentType: breakingContentType, action: authorization.ActionBreakingCheckRun, resource: authorization.Resource{Type: "module", Name: "user-api"}},
		{method: http.MethodGet, path: "/api/v1/modules/user-api/breaking-reports", action: authorization.ActionBreakingReportRead, resource: authorization.Resource{Type: "module", Name: "user-api"}},
		{method: http.MethodGet, path: "/api/v1/breaking-reports/report-1", action: authorization.ActionBreakingReportRead, resource: authorization.Resource{Type: "breaking_report", ID: "report-1"}},
		{method: http.MethodGet, path: "/api/v1/breaking-reports/report-1/affected-modules", action: authorization.ActionBreakingReportRead, resource: authorization.Resource{Type: "breaking_report", ID: "report-1"}},
		{method: http.MethodGet, path: "/api/v1/breaking-reports/report-1/runtime-impact", action: authorization.ActionBreakingReportRead, resource: authorization.Resource{Type: "breaking_report", ID: "report-1"}},
		{method: http.MethodPost, path: "/api/v1/breaking-reports/report-1/approval-request", body: jsonBody(`{}`), contentType: "application/json", action: authorization.ActionApprovalRequestCreate, resource: authorization.Resource{Type: "breaking_report", ID: "report-1"}},
		{method: http.MethodGet, path: "/api/v1/breaking-reports/report-1/approval-status", action: authorization.ActionApprovalRequestRead, resource: authorization.Resource{Type: "breaking_report", ID: "report-1"}},
		{method: http.MethodPost, path: "/api/v1/approval-requests/request-1/requirements/requirement-1/approve", body: jsonBody(`{"comment":"ok"}`), contentType: "application/json", action: authorization.ActionApprovalDecisionRecord, resource: authorization.Resource{Type: "approval_request", ID: "request-1"}},
		{method: http.MethodPost, path: "/api/v1/approval-requests/request-1/requirements/requirement-1/reject", body: jsonBody(`{"comment":"no"}`), contentType: "application/json", action: authorization.ActionApprovalDecisionRecord, resource: authorization.Resource{Type: "approval_request", ID: "request-1"}},
		{method: http.MethodGet, path: "/api/v1/approval-requests/request-1/audit", action: authorization.ActionGovernanceAuditRead, resource: authorization.Resource{Type: "approval_request", ID: "request-1"}},
		{method: http.MethodPost, path: "/api/v1/runtime/reports", body: jsonBody(`{"service_name":"billing-service","environment":"production","git_commit":"abc123","build_version":"v1","modules":[{"module":"user-api","version":"v1.0.0"}]}`), contentType: "application/json", action: authorization.ActionRuntimeInventoryReport, resource: authorization.Resource{Type: "runtime_inventory"}},
		{method: http.MethodGet, path: "/api/v1/runtime/services", action: authorization.ActionRuntimeInventoryRead, resource: authorization.Resource{Type: "runtime_inventory"}},
		{method: http.MethodGet, path: "/api/v1/runtime/services/billing-service", action: authorization.ActionRuntimeInventoryRead, resource: authorization.Resource{Type: "runtime_service", Name: "billing-service"}},
		{method: http.MethodGet, path: "/api/v1/runtime/environments/production", action: authorization.ActionRuntimeInventoryRead, resource: authorization.Resource{Type: "runtime_environment", Name: "production"}},
	}
}

func assertResource(t *testing.T, got authorization.Resource, want authorization.Resource) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resource = %#v, want %#v", got, want)
	}
}

func createHTTPModule(t *testing.T, handler http.Handler, name string) domain.Module {
	t.Helper()
	res := request(t, handler, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"`+name+`"}`), "Bearer valid", "application/json")
	if res.Code != http.StatusCreated {
		t.Fatalf("create module %s status = %d, body = %s", name, res.Code, res.Body.String())
	}
	var body moduleDTO
	decodeResponse(t, res, &body)
	moduleName, err := domain.NewModuleName(body.Name)
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	return domain.Module{
		ID:        domain.NewModuleID(body.ID),
		Name:      moduleName,
		CreatedAt: body.CreatedAt,
		UpdatedAt: body.UpdatedAt,
	}
}

func (fake *fakeRegistry) addVersion(t *testing.T, module domain.Module, versionValue string) domain.ModuleVersion {
	t.Helper()
	version, err := domain.NewVersion(versionValue)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	moduleVersion := domain.ModuleVersion{
		ID:        domain.NewModuleVersionID(module.Name.String() + "-" + versionValue),
		ModuleID:  module.ID,
		Version:   version,
		Status:    domain.ModuleVersionStatusPublished,
		Digest:    "sha256:" + module.Name.String() + "-" + versionValue,
		CreatedAt: fake.now,
	}
	fake.versions[module.Name.String()+":"+versionValue] = moduleVersion
	return moduleVersion
}
