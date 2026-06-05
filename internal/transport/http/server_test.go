package httptransport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/storage"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
	"github.com/alryzden/ProtoRadar/internal/usecase/runtimeinventory"
)

func TestUnauthorizedWithoutToken(t *testing.T) {
	server := newTestServer()

	res := request(t, server, http.MethodGet, "/api/v1/modules", nil, "", "")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
	assertAPIError(t, res, "unauthorized")
}

func TestValidationErrorResponseFormat(t *testing.T) {
	server := newTestServer()

	res := request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"bad name"}`), "Bearer valid", "application/json")
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
	assertAPIError(t, res, "validation_error")
}

func TestNotFoundErrorResponseFormat(t *testing.T) {
	server := newTestServer()

	res := request(t, server, http.MethodGet, "/api/v1/modules/missing-api", nil, "Bearer valid", "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
	assertAPIError(t, res, "not_found")
}

func TestHealthAndReadinessArePublic(t *testing.T) {
	server := NewServer(newFakeRegistry(), Options{
		BootstrapToken: "bootstrap",
		Ready: func(ctx context.Context) error {
			return nil
		},
	}).Handler()

	health := request(t, server, http.MethodGet, "/healthz", nil, "", "")
	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d", health.Code)
	}
	var healthBody map[string]string
	decodeResponse(t, health, &healthBody)
	if healthBody["status"] != "ok" {
		t.Fatalf("health body = %#v", healthBody)
	}
	ready := request(t, server, http.MethodGet, "/readyz", nil, "", "")
	if ready.Code != http.StatusOK {
		t.Fatalf("ready status = %d", ready.Code)
	}
	var readyBody readinessResponse
	decodeResponse(t, ready, &readyBody)
	if readyBody.Status != "ok" || readyBody.Checks["database"] != "ok" {
		t.Fatalf("ready body = %#v", readyBody)
	}
}

func TestReadinessFailure(t *testing.T) {
	server := NewServer(newFakeRegistry(), Options{
		BootstrapToken: "bootstrap",
		Ready: func(ctx context.Context) error {
			return errors.New("database unavailable")
		},
	}).Handler()

	res := request(t, server, http.MethodGet, "/readyz", nil, "", "")
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusServiceUnavailable)
	}
	var body readinessResponse
	decodeResponse(t, res, &body)
	if body.Status != "error" || body.Checks["database"] != "error" {
		t.Fatalf("body = %#v", body)
	}
	if strings.Contains(res.Body.String(), "database unavailable") {
		t.Fatalf("readiness leaked dependency error: %s", res.Body.String())
	}
}

func TestMetricsEndpointReturnsPrometheusText(t *testing.T) {
	server := NewServer(newFakeRegistry(), Options{
		BootstrapToken: "bootstrap",
		Metrics:        NewMetrics(),
	}).Handler()

	request(t, server, http.MethodGet, "/healthz", nil, "", "")
	res := request(t, server, http.MethodGet, "/metrics", nil, "", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if !strings.Contains(res.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("content type = %q", res.Header().Get("Content-Type"))
	}
	body := res.Body.String()
	if !strings.Contains(body, "protoradar_http_requests_total") || !strings.Contains(body, "protoradar_http_in_flight_requests") {
		t.Fatalf("metrics body missing expected series:\n%s", body)
	}
}

func TestRequestIDGeneratedWhenMissing(t *testing.T) {
	server := newTestServer()

	res := request(t, server, http.MethodGet, "/healthz", nil, "", "")
	if res.Header().Get(requestIDHeader) == "" {
		t.Fatalf("missing %s response header", requestIDHeader)
	}
}

func TestRequestIDPropagatedWhenProvided(t *testing.T) {
	server := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set(requestIDHeader, "request-123")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Header().Get(requestIDHeader) != "request-123" {
		t.Fatalf("request id = %q", res.Header().Get(requestIDHeader))
	}
}

func TestRequestLogsContainRequestIDAndRedactAuthorization(t *testing.T) {
	var logs bytes.Buffer
	logger, err := NewLogger("info", "json", &logs)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	server := NewServer(newFakeRegistry(), Options{
		BootstrapToken: "bootstrap",
		Logger:         logger,
	}).Handler()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set(requestIDHeader, "request-123")
	req.Header.Set("Authorization", "Bearer secret-token")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	body := logs.String()
	if !strings.Contains(body, `"request_id":"request-123"`) {
		t.Fatalf("log missing request id: %s", body)
	}
	if strings.Contains(body, "secret-token") || strings.Contains(body, "Authorization") {
		t.Fatalf("log leaked authorization data: %s", body)
	}
}

func TestMetricsUseRouteTemplateNotRawPath(t *testing.T) {
	fake := newFakeRegistry()
	server := NewServer(fake, Options{
		BootstrapToken: "bootstrap",
		Metrics:        NewMetrics(),
	}).Handler()
	createHTTPModule(t, server, "user-api")

	res := request(t, server, http.MethodGet, "/api/v1/modules/user-api", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	metrics := request(t, server, http.MethodGet, "/metrics", nil, "", "").Body.String()
	if !strings.Contains(metrics, `route="/api/v1/modules/{module}"`) {
		t.Fatalf("metrics missing route template:\n%s", metrics)
	}
	if strings.Contains(metrics, "user-api") {
		t.Fatalf("metrics leaked raw path label:\n%s", metrics)
	}
}

func TestUnauthorizedWithInvalidToken(t *testing.T) {
	server := newTestServer()

	res := request(t, server, http.MethodGet, "/api/v1/modules", nil, "Bearer invalid", "")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestPostModulesCreatesModule(t *testing.T) {
	server := newTestServer()

	res := request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{
		"name": "user-api",
		"description": "User service protobuf contracts",
		"repository_url": "https://gitlab.example.com/platform/user-api"
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}

	var body moduleDTO
	decodeResponse(t, res, &body)
	if body.Name != "user-api" {
		t.Fatalf("name = %q", body.Name)
	}
}

func TestPostModulesDuplicateMapsToConflict(t *testing.T) {
	server := newTestServer()
	body := strings.NewReader(`{"name":"user-api"}`)
	if res := request(t, server, http.MethodPost, "/api/v1/modules", body, "Bearer valid", "application/json"); res.Code != http.StatusCreated {
		t.Fatalf("first status = %d", res.Code)
	}

	res := request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")
	if res.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusConflict)
	}
	assertAPIError(t, res, "conflict")
}

func TestGetModulesReturnsList(t *testing.T) {
	server := newTestServer()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")

	res := request(t, server, http.MethodGet, "/api/v1/modules", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}

	var body listModulesResponse
	decodeResponse(t, res, &body)
	if len(body.Modules) != 1 || body.Modules[0].Name != "user-api" {
		t.Fatalf("modules = %#v", body.Modules)
	}
}

func TestPutModuleGitLabProjectUnauthorizedReturnsUnauthorized(t *testing.T) {
	server := newTestServer()

	res := request(t, server, http.MethodPut, "/api/v1/modules/user-api/gitlab-project", strings.NewReader(`{}`), "", "application/json")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestGetModuleGitLabProjectUnauthorizedReturnsUnauthorized(t *testing.T) {
	server := newTestServer()

	res := request(t, server, http.MethodGet, "/api/v1/modules/user-api/gitlab-project", nil, "", "")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestPutModuleGitLabProjectLinksMapping(t *testing.T) {
	server := newTestServer()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")

	res := request(t, server, http.MethodPut, "/api/v1/modules/user-api/gitlab-project", strings.NewReader(`{
		"gitlab_base_url": "https://gitlab.example.com",
		"gitlab_project_id": 12345,
		"gitlab_project_path": "platform/user-api"
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}

	var body moduleGitLabProjectDTO
	decodeResponse(t, res, &body)
	if body.Module != "user-api" {
		t.Fatalf("module = %q", body.Module)
	}
	if body.GitLabBaseURL != "https://gitlab.example.com" || body.GitLabProjectID != 12345 || body.GitLabProjectPath != "platform/user-api" {
		t.Fatalf("gitlab project = %#v", body)
	}
	if !body.CreatedAt.Equal(time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)) || !body.UpdatedAt.Equal(time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("timestamps = %s/%s", body.CreatedAt, body.UpdatedAt)
	}
}

func TestPutModuleGitLabProjectUpdatesMapping(t *testing.T) {
	server := newTestServer()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")
	request(t, server, http.MethodPut, "/api/v1/modules/user-api/gitlab-project", strings.NewReader(`{
		"gitlab_base_url": "https://gitlab.example.com",
		"gitlab_project_id": 12345,
		"gitlab_project_path": "platform/user-api"
	}`), "Bearer valid", "application/json")

	res := request(t, server, http.MethodPut, "/api/v1/modules/user-api/gitlab-project", strings.NewReader(`{
		"gitlab_base_url": "https://gitlab.example.org",
		"gitlab_project_id": 67890,
		"gitlab_project_path": "platform/user-api-v2"
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body moduleGitLabProjectDTO
	decodeResponse(t, res, &body)
	if body.GitLabBaseURL != "https://gitlab.example.org" || body.GitLabProjectID != 67890 || body.GitLabProjectPath != "platform/user-api-v2" {
		t.Fatalf("updated mapping = %#v", body)
	}
}

func TestPutModuleGitLabProjectInvalidBaseURLMapsToBadRequest(t *testing.T) {
	server := newTestServer()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")

	res := request(t, server, http.MethodPut, "/api/v1/modules/user-api/gitlab-project", strings.NewReader(`{
		"gitlab_base_url": "://bad-url",
		"gitlab_project_id": 12345,
		"gitlab_project_path": "platform/user-api"
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

func TestPutModuleGitLabProjectInvalidProjectIDMapsToBadRequest(t *testing.T) {
	server := newTestServer()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")

	res := request(t, server, http.MethodPut, "/api/v1/modules/user-api/gitlab-project", strings.NewReader(`{
		"gitlab_base_url": "https://gitlab.example.com",
		"gitlab_project_id": 0,
		"gitlab_project_path": "platform/user-api"
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

func TestPutModuleGitLabProjectEmptyProjectPathMapsToBadRequest(t *testing.T) {
	server := newTestServer()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")

	res := request(t, server, http.MethodPut, "/api/v1/modules/user-api/gitlab-project", strings.NewReader(`{
		"gitlab_base_url": "https://gitlab.example.com",
		"gitlab_project_id": 12345,
		"gitlab_project_path": " "
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

func TestPutModuleGitLabProjectUnknownModuleMapsToNotFound(t *testing.T) {
	server := newTestServer()

	res := request(t, server, http.MethodPut, "/api/v1/modules/missing/gitlab-project", strings.NewReader(`{
		"gitlab_base_url": "https://gitlab.example.com",
		"gitlab_project_id": 12345,
		"gitlab_project_path": "platform/missing"
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

func TestPutModuleGitLabProjectConflictMapsToConflict(t *testing.T) {
	server := newTestServer()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"billing-api"}`), "Bearer valid", "application/json")
	request(t, server, http.MethodPut, "/api/v1/modules/user-api/gitlab-project", strings.NewReader(`{
		"gitlab_base_url": "https://gitlab.example.com",
		"gitlab_project_id": 12345,
		"gitlab_project_path": "platform/user-api"
	}`), "Bearer valid", "application/json")

	res := request(t, server, http.MethodPut, "/api/v1/modules/billing-api/gitlab-project", strings.NewReader(`{
		"gitlab_base_url": "https://gitlab.example.com",
		"gitlab_project_id": 12345,
		"gitlab_project_path": "platform/user-api"
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusConflict)
	}
}

func TestGetModuleGitLabProjectReturnsMapping(t *testing.T) {
	server := newTestServer()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")
	request(t, server, http.MethodPut, "/api/v1/modules/user-api/gitlab-project", strings.NewReader(`{
		"gitlab_base_url": "https://gitlab.example.com",
		"gitlab_project_id": 12345,
		"gitlab_project_path": "platform/user-api"
	}`), "Bearer valid", "application/json")

	res := request(t, server, http.MethodGet, "/api/v1/modules/user-api/gitlab-project", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body moduleGitLabProjectDTO
	decodeResponse(t, res, &body)
	if body.Module != "user-api" || body.GitLabBaseURL != "https://gitlab.example.com" || body.GitLabProjectID != 12345 || body.GitLabProjectPath != "platform/user-api" {
		t.Fatalf("mapping = %#v", body)
	}
}

func TestGetModuleGitLabProjectUnknownModuleMapsToNotFound(t *testing.T) {
	server := newTestServer()

	res := request(t, server, http.MethodGet, "/api/v1/modules/missing/gitlab-project", nil, "Bearer valid", "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

func TestGetModuleGitLabProjectMissingMappingMapsToNotFound(t *testing.T) {
	server := newTestServer()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")

	res := request(t, server, http.MethodGet, "/api/v1/modules/user-api/gitlab-project", nil, "Bearer valid", "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

func TestDependencyGraphEndpointsRequireAuthorization(t *testing.T) {
	server := newTestServer()

	paths := []string{
		"/api/v1/modules/user-api/dependencies",
		"/api/v1/modules/user-api/affected",
		"/api/v1/breaking-reports/report-1/affected-modules",
	}
	for _, path := range paths {
		res := request(t, server, http.MethodGet, path, nil, "", "")
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want %d", path, res.Code, http.StatusUnauthorized)
		}
	}
}

func TestGetModuleDependenciesReturnsUpstreamAndDownstream(t *testing.T) {
	fake := newFakeRegistry()
	server := NewServer(fake, Options{BootstrapToken: "bootstrap"}).Handler()
	user := createHTTPModule(t, server, "user-api")
	billing := createHTTPModule(t, server, "billing-api")
	orders := createHTTPModule(t, server, "orders-api")
	userVersion := fake.addVersion(t, user, "v1.0.0")
	billingVersion := fake.addVersion(t, billing, "v1.4.0")
	ordersVersion := fake.addVersion(t, orders, "v2.0.0")
	fake.dependencies = []domain.ModuleDependency{
		testDependency(billing, billingVersion, user, userVersion, domain.DependencySourceImport, domain.DependencyResolutionReasonImportPath, "user/v1/user.proto", ""),
		testDependency(orders, ordersVersion, billing, billingVersion, domain.DependencySourceTypeReference, domain.DependencyResolutionReasonSymbol, "", "billing.v1.Invoice"),
	}
	fake.unresolved = []domain.UnresolvedProtoDependency{testUnresolved(user, userVersion, "missing/v1/missing.proto")}

	res := request(t, server, http.MethodGet, "/api/v1/modules/billing-api/dependencies", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}

	var body moduleDependencyGraphDTO
	decodeResponse(t, res, &body)
	if body.Module != "billing-api" {
		t.Fatalf("module = %q", body.Module)
	}
	if len(body.Upstream) != 1 || body.Upstream[0].Module != "user-api" || body.Upstream[0].LatestVersion != "v1.0.0" {
		t.Fatalf("upstream = %#v", body.Upstream)
	}
	if len(body.Downstream) != 1 || body.Downstream[0].Module != "orders-api" || body.Downstream[0].LatestVersion != "v2.0.0" {
		t.Fatalf("downstream = %#v", body.Downstream)
	}
	if !slices.Contains(body.Upstream[0].DependencySources, "import") || !slices.Contains(body.Upstream[0].Reasons, "imports user/v1/user.proto") {
		t.Fatalf("upstream dto missing source/reason: %#v", body.Upstream[0])
	}
	if !slices.Contains(body.Downstream[0].DependencySources, "type_reference") || !slices.Contains(body.Downstream[0].Reasons, "references billing.v1.Invoice") {
		t.Fatalf("downstream dto missing source/reason: %#v", body.Downstream[0])
	}
}

func TestGetModuleDependenciesReturnsEmptyListsWhenNoGraphExists(t *testing.T) {
	fake := newFakeRegistry()
	server := NewServer(fake, Options{BootstrapToken: "bootstrap"}).Handler()
	createHTTPModule(t, server, "user-api")

	res := request(t, server, http.MethodGet, "/api/v1/modules/user-api/dependencies", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body moduleDependencyGraphDTO
	decodeResponse(t, res, &body)
	if len(body.Upstream) != 0 || len(body.Downstream) != 0 || len(body.Unresolved) != 0 {
		t.Fatalf("body = %#v", body)
	}
}

func TestGetModuleDependenciesUnknownModuleReturnsNotFound(t *testing.T) {
	server := newTestServer()

	res := request(t, server, http.MethodGet, "/api/v1/modules/missing/dependencies", nil, "Bearer valid", "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

func TestGetAffectedModulesReturnsDirectDownstreamModules(t *testing.T) {
	fake := newFakeRegistry()
	server := NewServer(fake, Options{BootstrapToken: "bootstrap"}).Handler()
	user := createHTTPModule(t, server, "user-api")
	billing := createHTTPModule(t, server, "billing-api")
	userVersion := fake.addVersion(t, user, "v1.0.0")
	billingVersion := fake.addVersion(t, billing, "v1.4.0")
	fake.dependencies = []domain.ModuleDependency{
		testDependency(billing, billingVersion, user, userVersion, domain.DependencySourceImport, domain.DependencyResolutionReasonImportPath, "user/v1/user.proto", ""),
	}

	res := request(t, server, http.MethodGet, "/api/v1/modules/user-api/affected?depth=1", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body affectedModulesDTO
	decodeResponse(t, res, &body)
	if body.Module != "user-api" || len(body.AffectedModules) != 1 {
		t.Fatalf("body = %#v", body)
	}
	affected := body.AffectedModules[0]
	if affected.Module != "billing-api" || affected.LatestVersion != "v1.4.0" || !slices.Contains(affected.DependencySources, "import") || !slices.Contains(affected.Reasons, "import_path") {
		t.Fatalf("affected = %#v", affected)
	}
}

func TestGetAffectedModulesReturnsEmptyListWhenNoConsumersExist(t *testing.T) {
	fake := newFakeRegistry()
	server := NewServer(fake, Options{BootstrapToken: "bootstrap"}).Handler()
	createHTTPModule(t, server, "user-api")

	res := request(t, server, http.MethodGet, "/api/v1/modules/user-api/affected", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body affectedModulesDTO
	decodeResponse(t, res, &body)
	if body.Module != "user-api" || len(body.AffectedModules) != 0 {
		t.Fatalf("body = %#v", body)
	}
}

func TestGetBreakingReportAffectedModulesReturnsAffectedModules(t *testing.T) {
	fake := newFakeRegistry()
	server := NewServer(fake, Options{BootstrapToken: "bootstrap"}).Handler()
	user := createHTTPModule(t, server, "user-api")
	billing := createHTTPModule(t, server, "billing-api")
	userVersion := fake.addVersion(t, user, "v1.0.0")
	billingVersion := fake.addVersion(t, billing, "v1.4.0")
	fake.dependencies = []domain.ModuleDependency{
		testDependency(billing, billingVersion, user, userVersion, domain.DependencySourceImport, domain.DependencyResolutionReasonImportPath, "user/v1/user.proto", ""),
	}
	report := domain.BreakingReport{
		ID:          domain.NewBreakingReportID("report-1"),
		ModuleID:    user.ID,
		ModuleName:  user.Name,
		BaseVersion: userVersion.Version,
		Status:      domain.BreakingReportStatusBreaking,
		CreatedAt:   fake.now,
	}
	fake.reports[report.ID.String()] = registry.CheckBreakingResponse{Report: report}

	res := request(t, server, http.MethodGet, "/api/v1/breaking-reports/report-1/affected-modules", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body breakingReportAffectedModulesDTO
	decodeResponse(t, res, &body)
	if body.ReportID != "report-1" || body.Module != "user-api" || body.Status != "breaking" || len(body.AffectedModules) != 1 {
		t.Fatalf("body = %#v", body)
	}
}

func TestGetBreakingReportAffectedModulesNotFoundReturnsNotFound(t *testing.T) {
	server := newTestServer()

	res := request(t, server, http.MethodGet, "/api/v1/breaking-reports/missing/affected-modules", nil, "Bearer valid", "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

func TestDependencyGraphInternalErrorDoesNotExposeStackTrace(t *testing.T) {
	fake := newFakeRegistry()
	server := NewServer(fake, Options{BootstrapToken: "bootstrap"}).Handler()
	createHTTPModule(t, server, "user-api")
	fake.dependencyErr = errors.New("resolver failed\nstack trace: secret.go:42")

	res := request(t, server, http.MethodGet, "/api/v1/modules/user-api/dependencies", nil, "Bearer valid", "")
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusInternalServerError)
	}
	if strings.Contains(res.Body.String(), "stack trace") || strings.Contains(res.Body.String(), "secret.go") {
		t.Fatalf("error response leaked internals: %s", res.Body.String())
	}
}

func TestRuntimeReportUnauthorizedReturnsUnauthorized(t *testing.T) {
	server := newRuntimeTestServer()

	res := request(t, server, http.MethodPost, "/api/v1/runtime/reports", strings.NewReader(`{}`), "", "application/json")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestPostRuntimeReportSuccess(t *testing.T) {
	fake := newFakeRegistry()
	server := newRuntimeTestServerWithFake(fake)
	user := createHTTPModule(t, server, "user-api")
	fake.addVersion(t, user, "v1.2.0")

	res := request(t, server, http.MethodPost, "/api/v1/runtime/reports", strings.NewReader(`{
		"service_name": "billing-service",
		"environment": "production",
		"git_commit": "abc1234",
		"build_version": "2026.06.04-15",
		"modules": [{"module": "user-api", "version": "v1.2.0"}]
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}

	var body reportRuntimeInventoryDTO
	decodeResponse(t, res, &body)
	if body.DeploymentID == "" || body.ServiceName != "billing-service" || body.Environment != "production" {
		t.Fatalf("body = %#v", body)
	}
	if len(body.Usages) != 1 || body.Usages[0].Module != "user-api" || body.Usages[0].DriftStatus != "up_to_date" {
		t.Fatalf("usages = %#v", body.Usages)
	}
}

func TestPostRuntimeReportInvalidJSONReturnsBadRequest(t *testing.T) {
	server := newRuntimeTestServer()

	res := request(t, server, http.MethodPost, "/api/v1/runtime/reports", strings.NewReader(`{bad json`), "Bearer valid", "application/json")
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

func TestPostRuntimeReportOversizedBodyReturnsPayloadTooLarge(t *testing.T) {
	fake := newFakeRegistry()
	server := NewServer(fake, Options{
		BootstrapToken:      "bootstrap",
		Runtime:             fake,
		MaxRequestBodyBytes: 8,
	}).Handler()

	res := request(t, server, http.MethodPost, "/api/v1/runtime/reports", strings.NewReader(`{"service_name":"billing-service"}`), "Bearer valid", "application/json")
	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusRequestEntityTooLarge)
	}
	assertAPIError(t, res, "payload_too_large")
}

func TestPostRuntimeReportMissingFieldsReturnBadRequest(t *testing.T) {
	server := newRuntimeTestServer()
	cases := []struct {
		name string
		body string
	}{
		{name: "missing service", body: `{"environment":"production","git_commit":"abc123","build_version":"build-1","modules":[{"module":"user-api","version":"v1.0.0"}]}`},
		{name: "missing environment", body: `{"service_name":"billing-service","git_commit":"abc123","build_version":"build-1","modules":[{"module":"user-api","version":"v1.0.0"}]}`},
		{name: "empty modules", body: `{"service_name":"billing-service","environment":"production","git_commit":"abc123","build_version":"build-1","modules":[]}`},
	}
	for _, tc := range cases {
		res := request(t, server, http.MethodPost, "/api/v1/runtime/reports", strings.NewReader(tc.body), "Bearer valid", "application/json")
		if res.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want %d, body = %s", tc.name, res.Code, http.StatusBadRequest, res.Body.String())
		}
	}
}

func TestPostRuntimeReportUnknownModuleUsageSucceeds(t *testing.T) {
	server := newRuntimeTestServer()

	res := request(t, server, http.MethodPost, "/api/v1/runtime/reports", strings.NewReader(`{
		"service_name": "billing-service",
		"environment": "production",
		"git_commit": "abc1234",
		"build_version": "build-1",
		"modules": [{"module": "missing-api", "version": "v1.0.0"}]
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body reportRuntimeInventoryDTO
	decodeResponse(t, res, &body)
	if len(body.Usages) != 1 || body.Usages[0].DriftStatus != "unknown_version" || body.Usages[0].DriftReason != "module_not_found" {
		t.Fatalf("usages = %#v", body.Usages)
	}
}

func TestPostRuntimeReportBehindLatestReturnsDriftStatus(t *testing.T) {
	fake := newFakeRegistry()
	server := newRuntimeTestServerWithFake(fake)
	user := createHTTPModule(t, server, "user-api")
	fake.addVersion(t, user, "v1.2.0")
	fake.now = fake.now.Add(time.Hour)
	fake.addVersion(t, user, "v1.5.0")

	res := request(t, server, http.MethodPost, "/api/v1/runtime/reports", strings.NewReader(`{
		"service_name": "billing-service",
		"environment": "production",
		"git_commit": "abc1234",
		"build_version": "build-1",
		"modules": [{"module": "user-api", "version": "v1.2.0"}]
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body reportRuntimeInventoryDTO
	decodeResponse(t, res, &body)
	if len(body.Usages) != 1 || body.Usages[0].DriftStatus != "behind_latest" || body.Usages[0].LatestVersion != "v1.5.0" {
		t.Fatalf("usages = %#v", body.Usages)
	}
}

func TestGetRuntimeServicesReturnsSummaries(t *testing.T) {
	fake := newFakeRegistry()
	server := newRuntimeTestServerWithFake(fake)
	fake.seedRuntimeServiceSummary(t, "billing-service", "production")

	res := request(t, server, http.MethodGet, "/api/v1/runtime/services", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body listRuntimeServicesResponse
	decodeResponse(t, res, &body)
	if len(body.Services) != 1 || body.Services[0].ServiceName != "billing-service" || !slices.Contains(body.Services[0].Environments, "production") {
		t.Fatalf("body = %#v", body)
	}
}

func TestGetRuntimeServiceDetailsReturnsDeploymentsAndUsages(t *testing.T) {
	fake := newFakeRegistry()
	server := newRuntimeTestServerWithFake(fake)
	fake.seedRuntimeDeployment(t, "billing-service", "production", "user-api", "v1.2.0")

	res := request(t, server, http.MethodGet, "/api/v1/runtime/services/billing-service", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body runtimeServiceDetailsDTO
	decodeResponse(t, res, &body)
	if body.ServiceName != "billing-service" || len(body.Deployments) != 1 || len(body.Usages) != 1 {
		t.Fatalf("body = %#v", body)
	}
}

func TestGetRuntimeServiceDetailsUnknownServiceReturnsNotFound(t *testing.T) {
	server := newRuntimeTestServer()

	res := request(t, server, http.MethodGet, "/api/v1/runtime/services/missing-service", nil, "Bearer valid", "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

func TestGetRuntimeEnvironmentInventoryReturnsInventory(t *testing.T) {
	fake := newFakeRegistry()
	server := newRuntimeTestServerWithFake(fake)
	fake.seedRuntimeDeployment(t, "billing-service", "production", "user-api", "v1.2.0")

	res := request(t, server, http.MethodGet, "/api/v1/runtime/environments/production", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body runtimeEnvironmentInventoryDTO
	decodeResponse(t, res, &body)
	if body.Environment != "production" || len(body.Deployments) != 1 || len(body.Usages) != 1 {
		t.Fatalf("body = %#v", body)
	}
}

func TestGetModuleRuntimeUsagesReturnsUsages(t *testing.T) {
	fake := newFakeRegistry()
	server := newRuntimeTestServerWithFake(fake)
	createHTTPModule(t, server, "user-api")
	fake.seedRuntimeDeployment(t, "billing-service", "production", "user-api", "v1.2.0")

	res := request(t, server, http.MethodGet, "/api/v1/modules/user-api/runtime-usages", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body moduleRuntimeUsagesDTO
	decodeResponse(t, res, &body)
	if body.Module != "user-api" || len(body.Usages) != 1 || body.Usages[0].ServiceName != "billing-service" {
		t.Fatalf("body = %#v", body)
	}
}

func TestGetBreakingReportRuntimeImpactReturnsImpacts(t *testing.T) {
	fake := newFakeRegistry()
	server := newRuntimeTestServerWithFake(fake)
	user := createHTTPModule(t, server, "user-api")
	version := fake.addVersion(t, user, "v1.2.0")
	report := domain.BreakingReport{
		ID:            domain.NewBreakingReportID("report-1"),
		ModuleID:      user.ID,
		ModuleName:    user.Name,
		BaseVersionID: version.ID,
		BaseVersion:   version.Version,
		Status:        domain.BreakingReportStatusBreaking,
		CreatedAt:     fake.now,
	}
	fake.reports[report.ID.String()] = registry.CheckBreakingResponse{Report: report}
	fake.seedRuntimeDeployment(t, "billing-service", "production", "user-api", "v1.2.0")

	res := request(t, server, http.MethodGet, "/api/v1/breaking-reports/report-1/runtime-impact", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body breakingReportRuntimeImpactDTO
	decodeResponse(t, res, &body)
	if body.ReportID != "report-1" || len(body.Impacts) != 1 || body.Impacts[0].ImpactStatus != "potentially_affected_by_breaking_change" {
		t.Fatalf("body = %#v", body)
	}
}

func TestGetBreakingReportRuntimeImpactUnknownReportReturnsNotFound(t *testing.T) {
	server := newRuntimeTestServer()

	res := request(t, server, http.MethodGet, "/api/v1/breaking-reports/missing/runtime-impact", nil, "Bearer valid", "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

func TestRuntimeInternalErrorDoesNotExposeStackTrace(t *testing.T) {
	fake := newFakeRegistry()
	fake.runtimeErr = errors.New("runtime failed\nstack trace: secret.go:42")
	server := newRuntimeTestServerWithFake(fake)

	res := request(t, server, http.MethodGet, "/api/v1/runtime/services", nil, "Bearer valid", "")
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusInternalServerError)
	}
	if strings.Contains(res.Body.String(), "stack trace") || strings.Contains(res.Body.String(), "secret.go") {
		t.Fatalf("error response leaked internals: %s", res.Body.String())
	}
}

func TestPublishModuleVersion(t *testing.T) {
	server := newTestServer()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")

	res := multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}

	var body publishModuleVersionResponse
	decodeResponse(t, res, &body)
	if body.Module != "user-api" || body.Version != "v1.0.0" {
		t.Fatalf("body = %#v", body)
	}
	if body.SourceArtifact.Kind != domain.ArtifactKindSourceArchive.String() {
		t.Fatalf("source kind = %q", body.SourceArtifact.Kind)
	}
	if body.BufImageArtifact.Kind != domain.ArtifactKindBufImage.String() {
		t.Fatalf("buf image kind = %q", body.BufImageArtifact.Kind)
	}
	if body.Buf.LintStatus != domain.BufLintStatusPassed.String() {
		t.Fatalf("lint status = %q", body.Buf.LintStatus)
	}
	if body.MetadataSummary.Files != 1 || body.MetadataSummary.Services != 1 {
		t.Fatalf("metadata summary = %#v", body.MetadataSummary)
	}
}

func TestPublishModuleVersionReturnsLintWarning(t *testing.T) {
	fake := newFakeRegistry()
	fake.publishLint = domain.BufLintResult{
		Status: domain.BufLintStatusWarning,
		Report: "lint warning",
	}
	server := NewServer(fake, Options{BootstrapToken: "bootstrap"}).Handler()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")

	res := multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}

	var body publishModuleVersionResponse
	decodeResponse(t, res, &body)
	if body.Buf.LintStatus != domain.BufLintStatusWarning.String() {
		t.Fatalf("lint status = %q", body.Buf.LintStatus)
	}
	if body.Buf.LintReport != "lint warning" {
		t.Fatalf("lint report = %q", body.Buf.LintReport)
	}
}

func TestPublishDuplicateVersionMapsToConflict(t *testing.T) {
	server := newTestServer()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")
	multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))

	res := multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))
	if res.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusConflict)
	}
}

func TestPublishUnknownModuleMapsToNotFound(t *testing.T) {
	server := newTestServer()

	res := multipartRequest(t, server, "/api/v1/modules/missing/versions", "v1.0.0", []byte("artifact"))
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

func TestPublishBufBuildFailureMapsToUnprocessableEntity(t *testing.T) {
	server := newServerWithPublishError(registry.ErrBufBuildFailed)

	res := multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))
	if res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnprocessableEntity)
	}
}

func TestPublishLintEnforceFailureMapsToUnprocessableEntity(t *testing.T) {
	server := newServerWithPublishError(registry.ErrBufLintFailed)

	res := multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))
	if res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnprocessableEntity)
	}
}

func TestPublishUnsafeArchiveMapsToBadRequest(t *testing.T) {
	server := newServerWithPublishError(registry.ErrUnsafeArchive)

	res := multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

func TestPublishArtifactTooLargeMapsToPayloadTooLarge(t *testing.T) {
	server := newServerWithPublishError(registry.ErrArtifactTooLarge)

	res := multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))
	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestPublishOversizedRequestBodyMapsToPayloadTooLarge(t *testing.T) {
	fake := newFakeRegistry()
	server := NewServer(fake, Options{
		BootstrapToken:      "bootstrap",
		Runtime:             fake,
		MaxRequestBodyBytes: 16,
	}).Handler()
	createHTTPModule(t, server, "user-api")

	res := multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))
	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusRequestEntityTooLarge)
	}
	assertAPIError(t, res, "payload_too_large")
}

func TestGetModuleVersionIncludesArtifactsAndMetadataSummary(t *testing.T) {
	server := newTestServer()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")
	multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))

	res := request(t, server, http.MethodGet, "/api/v1/modules/user-api/versions/v1.0.0", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}

	var body moduleVersionDetailsDTO
	decodeResponse(t, res, &body)
	if len(body.Artifacts) != 2 {
		t.Fatalf("artifacts = %#v", body.Artifacts)
	}
	if body.Buf.ConfigPresent != true || body.Buf.LockPresent != true {
		t.Fatalf("buf = %#v", body.Buf)
	}
	if body.MetadataSummary.Files != 1 || body.MetadataSummary.Messages != 1 {
		t.Fatalf("metadata summary = %#v", body.MetadataSummary)
	}
}

func TestGetModuleVersionMetadataReturnsDescriptorMetadata(t *testing.T) {
	server := newTestServer()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")
	multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))

	res := request(t, server, http.MethodGet, "/api/v1/modules/user-api/versions/v1.0.0/metadata", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}

	var body descriptorMetadataDTO
	decodeResponse(t, res, &body)
	if len(body.Files) != 1 {
		t.Fatalf("files = %#v", body.Files)
	}
	file := body.Files[0]
	if file.Path != "user/v1/user.proto" || len(file.Imports) != 1 {
		t.Fatalf("file = %#v", file)
	}
	if len(file.Services) != 1 || len(file.Services[0].Methods) != 1 {
		t.Fatalf("services = %#v", file.Services)
	}
	if len(file.Messages) != 1 || len(file.Messages[0].Fields) != 1 {
		t.Fatalf("messages = %#v", file.Messages)
	}
	if len(file.Enums) != 1 || len(file.Enums[0].Values) != 1 {
		t.Fatalf("enums = %#v", file.Enums)
	}
}

func TestPostBreakingCheckUnauthorizedReturnsUnauthorized(t *testing.T) {
	server := newTestServer()

	res := request(t, server, http.MethodPost, "/api/v1/modules/user-api/breaking-checks", nil, "", "")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestPostBreakingCheckReturnsPassed(t *testing.T) {
	server := newTestServer()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")
	multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))

	res := multipartBreakingRequest(t, server, "/api/v1/modules/user-api/breaking-checks", "v1.0.0", "local", []byte("source"))
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}

	var body breakingReportDTO
	decodeResponse(t, res, &body)
	if body.Module != "user-api" || body.Against != "v1.0.0" || body.Status != domain.BreakingReportStatusPassed.String() {
		t.Fatalf("body = %#v", body)
	}
	if !strings.Contains(body.HumanSummary, "ProtoRadar Breaking Change Report") {
		t.Fatalf("human summary = %q", body.HumanSummary)
	}
}

func TestPostBreakingCheckReturnsBreakingAsOK(t *testing.T) {
	fake := newFakeRegistry()
	fake.breakingStatus = domain.BreakingReportStatusBreaking
	fake.breakingChanges = []domain.BreakingChange{
		{
			Category:    "FIELD",
			FilePath:    "user/v1/user.proto",
			PackageName: "user.v1",
			Symbol:      "user.v1.User.email",
			RuleID:      "FIELD_SAME_TYPE",
			Message:     `Field "email" changed type from string to bytes.`,
			Severity:    "error",
		},
	}
	server := NewServer(fake, Options{BootstrapToken: "bootstrap"}).Handler()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")
	multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))

	res := multipartBreakingRequest(t, server, "/api/v1/modules/user-api/breaking-checks", "v1.0.0", "local", []byte("source"))
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}

	var body breakingReportDTO
	decodeResponse(t, res, &body)
	if body.Status != domain.BreakingReportStatusBreaking.String() || body.ChangeCount != 1 {
		t.Fatalf("body = %#v", body)
	}
	if len(body.Changes) != 1 || body.Changes[0].RuleID != "FIELD_SAME_TYPE" {
		t.Fatalf("changes = %#v", body.Changes)
	}
}

func TestPostBreakingCheckUnknownModuleMapsToNotFound(t *testing.T) {
	server := newServerWithCheckError(registry.ErrModuleNotFound)

	res := multipartBreakingRequest(t, server, "/api/v1/modules/user-api/breaking-checks", "v1.0.0", "local", []byte("source"))
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

func TestPostBreakingCheckUnknownBaselineMapsToNotFound(t *testing.T) {
	server := newServerWithCheckError(registry.ErrBaselineVersionNotFound)

	res := multipartBreakingRequest(t, server, "/api/v1/modules/user-api/breaking-checks", "v9.9.9", "local", []byte("source"))
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

func TestPostBreakingCheckBaselineWithoutBufImageMapsToConflict(t *testing.T) {
	server := newServerWithCheckError(registry.ErrBaselineBufImageMissing)

	res := multipartBreakingRequest(t, server, "/api/v1/modules/user-api/breaking-checks", "v1.0.0", "local", []byte("source"))
	if res.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusConflict)
	}
}

func TestPostBreakingCheckUnsafeArchiveMapsToBadRequest(t *testing.T) {
	server := newServerWithCheckError(registry.ErrUnsafeArchive)

	res := multipartBreakingRequest(t, server, "/api/v1/modules/user-api/breaking-checks", "v1.0.0", "local", []byte("source"))
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

func TestPostBreakingCheckArtifactTooLargeMapsToPayloadTooLarge(t *testing.T) {
	server := newServerWithCheckError(registry.ErrArtifactTooLarge)

	res := multipartBreakingRequest(t, server, "/api/v1/modules/user-api/breaking-checks", "v1.0.0", "local", []byte("source"))
	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestPostBreakingCheckMissingBufYAMLMapsToUnprocessableEntity(t *testing.T) {
	server := newServerWithCheckError(registry.ErrBufConfigNotFound)

	res := multipartBreakingRequest(t, server, "/api/v1/modules/user-api/breaking-checks", "v1.0.0", "local", []byte("source"))
	if res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnprocessableEntity)
	}
}

func TestGetBreakingReportReturnsReportWithChanges(t *testing.T) {
	fake := newFakeRegistry()
	fake.breakingStatus = domain.BreakingReportStatusBreaking
	fake.breakingChanges = []domain.BreakingChange{{FilePath: "user.proto", RuleID: "FIELD_NO_DELETE", Message: "field deleted", Severity: "error"}}
	server := NewServer(fake, Options{BootstrapToken: "bootstrap"}).Handler()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")
	multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))
	create := multipartBreakingRequest(t, server, "/api/v1/modules/user-api/breaking-checks", "v1.0.0", "local", []byte("source"))
	var created breakingReportDTO
	decodeResponse(t, create, &created)

	res := request(t, server, http.MethodGet, "/api/v1/breaking-reports/"+created.ID, nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body breakingReportDTO
	decodeResponse(t, res, &body)
	if body.ID != created.ID || len(body.Changes) != 1 {
		t.Fatalf("body = %#v", body)
	}
}

func TestListBreakingReportsReturnsModuleReports(t *testing.T) {
	fake := newFakeRegistry()
	server := NewServer(fake, Options{BootstrapToken: "bootstrap"}).Handler()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")
	multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))
	multipartBreakingRequest(t, server, "/api/v1/modules/user-api/breaking-checks", "v1.0.0", "first", []byte("source"))
	fake.now = fake.now.Add(time.Minute)
	multipartBreakingRequest(t, server, "/api/v1/modules/user-api/breaking-checks", "v1.0.0", "second", []byte("source"))

	res := request(t, server, http.MethodGet, "/api/v1/modules/user-api/breaking-reports", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body listBreakingReportsResponse
	decodeResponse(t, res, &body)
	if len(body.Reports) != 2 {
		t.Fatalf("reports = %#v", body.Reports)
	}
	if body.Reports[0].TargetRef != "second" || body.Reports[1].TargetRef != "first" {
		t.Fatalf("reports not ordered desc: %#v", body.Reports)
	}
}

func TestGetArtifactStreamsArtifact(t *testing.T) {
	server := newTestServer()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")
	multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))

	res := request(t, server, http.MethodGet, "/api/v1/modules/user-api/versions/v1.0.0/artifact", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if res.Body.String() != "artifact" {
		t.Fatalf("body = %q", res.Body.String())
	}
	if res.Header().Get("X-ProtoRadar-Digest") == "" {
		t.Fatalf("missing digest header")
	}
	if res.Header().Get("X-ProtoRadar-Checksum-SHA256") == "" {
		t.Fatalf("missing checksum header")
	}
}

func TestTokenEndpointDoesNotReturnTokenHash(t *testing.T) {
	server := newTestServer()

	res := request(t, server, http.MethodPost, "/api/v1/tokens", strings.NewReader(`{"name":"ci"}`), "Bearer bootstrap", "application/json")
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}

	var body map[string]any
	decodeResponse(t, res, &body)
	if _, exists := body["token_hash"]; exists {
		t.Fatalf("token_hash leaked in response")
	}
	if body["token"] != "raw-token" {
		t.Fatalf("token = %#v", body["token"])
	}
	if strings.Contains(res.Body.String(), "stored-hash") {
		t.Fatalf("stored hash leaked in response")
	}
}

func newTestServer() http.Handler {
	fake := newFakeRegistry()
	return NewServer(fake, Options{BootstrapToken: "bootstrap", Runtime: fake}).Handler()
}

func newServerWithPublishError(err error) http.Handler {
	fake := newFakeRegistry()
	fake.publishErr = err
	_, _ = fake.CreateModule(context.Background(), registry.CreateModuleRequest{Name: "user-api"})
	return NewServer(fake, Options{BootstrapToken: "bootstrap", Runtime: fake}).Handler()
}

func newServerWithCheckError(err error) http.Handler {
	fake := newFakeRegistry()
	fake.checkErr = err
	_, _ = fake.CreateModule(context.Background(), registry.CreateModuleRequest{Name: "user-api"})
	return NewServer(fake, Options{BootstrapToken: "bootstrap", Runtime: fake}).Handler()
}

func newRuntimeTestServer() http.Handler {
	return newRuntimeTestServerWithFake(newFakeRegistry())
}

func newRuntimeTestServerWithFake(fake *fakeRegistry) http.Handler {
	return NewServer(fake, Options{BootstrapToken: "bootstrap", Runtime: fake}).Handler()
}

func request(t *testing.T, handler http.Handler, method string, path string, body io.Reader, auth string, contentType string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, body)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func multipartRequest(t *testing.T, handler http.Handler, path string, version string, artifact []byte) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("version", version); err != nil {
		t.Fatalf("write version: %v", err)
	}
	file, err := writer.CreateFormFile("artifact", "artifact.tar.gz")
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	if _, err := file.Write(artifact); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	return request(t, handler, http.MethodPost, path, &body, "Bearer valid", writer.FormDataContentType())
}

func multipartBreakingRequest(t *testing.T, handler http.Handler, path string, against string, targetRef string, artifact []byte) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("against", against); err != nil {
		t.Fatalf("write against: %v", err)
	}
	if targetRef != "" {
		if err := writer.WriteField("target_ref", targetRef); err != nil {
			t.Fatalf("write target ref: %v", err)
		}
	}
	file, err := writer.CreateFormFile("artifact", "source.tar.gz")
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	if _, err := file.Write(artifact); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	return request(t, handler, http.MethodPost, path, &body, "Bearer valid", writer.FormDataContentType())
}

func decodeResponse(t *testing.T, res *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.NewDecoder(res.Body).Decode(dst); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func assertAPIError(t *testing.T, res *httptest.ResponseRecorder, wantCode string) {
	t.Helper()
	var body errorResponse
	decodeResponse(t, res, &body)
	if body.Error.Code != wantCode {
		t.Fatalf("error code = %q, want %q; body = %#v", body.Error.Code, wantCode, body)
	}
	if strings.TrimSpace(body.Error.Message) == "" {
		t.Fatalf("error message is empty: %#v", body)
	}
	for _, forbidden := range []string{"stack trace", "secret.go", "panic:", "pq:", "SQLSTATE"} {
		if strings.Contains(body.Error.Message, forbidden) {
			t.Fatalf("error message leaked %q: %#v", forbidden, body)
		}
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

func testDependency(consumer domain.Module, consumerVersion domain.ModuleVersion, provider domain.Module, providerVersion domain.ModuleVersion, source domain.DependencySource, reason domain.DependencyResolutionReason, importPath string, symbol string) domain.ModuleDependency {
	return domain.ModuleDependency{
		ID:                      domain.NewModuleDependencyID(consumer.Name.String() + "-" + provider.Name.String() + "-" + source.String()),
		ConsumerModuleID:        consumer.ID,
		ConsumerModuleName:      consumer.Name,
		ConsumerModuleVersionID: consumerVersion.ID,
		ConsumerVersion:         consumerVersion.Version,
		ProviderModuleID:        provider.ID,
		ProviderModuleName:      provider.Name,
		ProviderModuleVersionID: providerVersion.ID,
		ProviderVersion:         providerVersion.Version,
		Source:                  source,
		Reason:                  reason,
		ImportPath:              importPath,
		ReferencedSymbol:        symbol,
	}
}

func testUnresolved(module domain.Module, moduleVersion domain.ModuleVersion, importPath string) domain.UnresolvedProtoDependency {
	return domain.UnresolvedProtoDependency{
		ID:              domain.NewUnresolvedProtoDependencyID("unresolved-" + module.Name.String()),
		ModuleID:        module.ID,
		ModuleName:      module.Name,
		ModuleVersionID: moduleVersion.ID,
		Version:         moduleVersion.Version,
		Source:          domain.DependencySourceImport,
		ImportPath:      importPath,
		Reason:          domain.UnresolvedDependencyReasonProviderNotFound,
	}
}

type fakeRegistry struct {
	now                time.Time
	modules            map[string]domain.Module
	versions           map[string]domain.ModuleVersion
	artifacts          map[string][]domain.Artifact
	objects            map[string][]byte
	bufConfigs         map[string]domain.BufConfigInfo
	lintResults        map[string]domain.BufLintResult
	metadata           map[string]domain.DescriptorMetadata
	reports            map[string]registry.CheckBreakingResponse
	gitLabProjects     map[string]domain.ModuleGitLabProject
	dependencies       []domain.ModuleDependency
	unresolved         []domain.UnresolvedProtoDependency
	runtimeServices    map[string]domain.RuntimeService
	runtimeDeployments []domain.RuntimeDeployment
	runtimeUsages      []domain.RuntimeModuleUsage
	dependencyErr      error
	runtimeErr         error
	publishErr         error
	linkGitLabErr      error
	publishLint        domain.BufLintResult
	checkErr           error
	breakingStatus     domain.BreakingReportStatus
	breakingChanges    []domain.BreakingChange
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{
		now:             time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC),
		modules:         map[string]domain.Module{},
		versions:        map[string]domain.ModuleVersion{},
		artifacts:       map[string][]domain.Artifact{},
		objects:         map[string][]byte{},
		bufConfigs:      map[string]domain.BufConfigInfo{},
		lintResults:     map[string]domain.BufLintResult{},
		metadata:        map[string]domain.DescriptorMetadata{},
		reports:         map[string]registry.CheckBreakingResponse{},
		gitLabProjects:  map[string]domain.ModuleGitLabProject{},
		runtimeServices: map[string]domain.RuntimeService{},
		publishLint:     domain.BufLintResult{Status: domain.BufLintStatusPassed},
		breakingStatus:  domain.BreakingReportStatusPassed,
	}
}

func (fake *fakeRegistry) AuthenticateToken(ctx context.Context, rawToken string) (registry.AuthSubject, error) {
	if rawToken != "valid" {
		return registry.AuthSubject{}, registry.ErrInvalidOrExpiredToken
	}
	return registry.AuthSubject{Name: "test"}, nil
}

func (fake *fakeRegistry) CreateModule(ctx context.Context, req registry.CreateModuleRequest) (domain.Module, error) {
	name, err := domain.NewModuleName(req.Name)
	if err != nil {
		return domain.Module{}, registry.ErrInvalidModuleName
	}
	if _, exists := fake.modules[name.String()]; exists {
		return domain.Module{}, registry.ErrModuleAlreadyExists
	}
	module := domain.Module{
		ID:            domain.NewModuleID("module-" + name.String()),
		Name:          name,
		Description:   req.Description,
		RepositoryURL: req.RepositoryURL,
		CreatedAt:     fake.now,
		UpdatedAt:     fake.now,
	}
	fake.modules[name.String()] = module
	return module, nil
}

func (fake *fakeRegistry) ListModules(ctx context.Context, limit int, offset int) ([]domain.Module, error) {
	modules := make([]domain.Module, 0, len(fake.modules))
	for _, module := range fake.modules {
		modules = append(modules, module)
	}
	return modules, nil
}

func (fake *fakeRegistry) GetModule(ctx context.Context, name string) (domain.Module, error) {
	module, exists := fake.modules[name]
	if !exists {
		return domain.Module{}, registry.ErrModuleNotFound
	}
	return module, nil
}

func (fake *fakeRegistry) LinkModuleGitLabProject(ctx context.Context, input registry.LinkModuleGitLabProjectInput) (registry.LinkModuleGitLabProjectOutput, error) {
	if fake.linkGitLabErr != nil {
		return registry.LinkModuleGitLabProjectOutput{}, fake.linkGitLabErr
	}
	module, exists := fake.modules[input.ModuleName]
	if !exists {
		return registry.LinkModuleGitLabProjectOutput{}, registry.ErrModuleNotFound
	}
	mapping := domain.ModuleGitLabProject{
		ID:                domain.NewModuleGitLabProjectID("gitlab-project-" + input.ModuleName),
		ModuleID:          module.ID,
		ModuleName:        module.Name,
		GitLabBaseURL:     input.GitLabBaseURL,
		GitLabProjectID:   input.GitLabProjectID,
		GitLabProjectPath: input.GitLabProjectPath,
		CreatedAt:         fake.now,
		UpdatedAt:         fake.now,
	}
	normalized, err := mapping.Normalized()
	if err != nil {
		return registry.LinkModuleGitLabProjectOutput{}, mapFakeGitLabValidationError(err)
	}
	if existing, exists := fake.gitLabProjectByBaseURLAndID(normalized.GitLabBaseURL, normalized.GitLabProjectID); exists && existing.ModuleID != module.ID {
		return registry.LinkModuleGitLabProjectOutput{}, registry.ErrGitLabProjectAlreadyLinked
	}
	if existing, exists := fake.gitLabProjects[module.ID.String()]; exists {
		normalized.ID = existing.ID
		normalized.CreatedAt = existing.CreatedAt
	}
	fake.gitLabProjects[module.ID.String()] = normalized
	return registry.LinkModuleGitLabProjectOutput{Mapping: normalized}, nil
}

func (fake *fakeRegistry) GetModuleGitLabProject(ctx context.Context, moduleName string) (registry.GetModuleGitLabProjectOutput, error) {
	module, exists := fake.modules[moduleName]
	if !exists {
		return registry.GetModuleGitLabProjectOutput{}, registry.ErrModuleNotFound
	}
	mapping, exists := fake.gitLabProjects[module.ID.String()]
	if !exists {
		return registry.GetModuleGitLabProjectOutput{}, registry.ErrModuleGitLabProjectNotFound
	}
	return registry.GetModuleGitLabProjectOutput{Mapping: mapping}, nil
}

func (fake *fakeRegistry) gitLabProjectByBaseURLAndID(baseURL string, projectID int64) (domain.ModuleGitLabProject, bool) {
	for _, mapping := range fake.gitLabProjects {
		if mapping.GitLabBaseURL == baseURL && mapping.GitLabProjectID == projectID {
			return mapping, true
		}
	}
	return domain.ModuleGitLabProject{}, false
}

func mapFakeGitLabValidationError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidModuleName):
		return registry.ErrInvalidModuleName
	case errors.Is(err, domain.ErrInvalidGitLabBaseURL):
		return registry.ErrInvalidGitLabBaseURL
	case errors.Is(err, domain.ErrInvalidGitLabProjectID):
		return registry.ErrInvalidGitLabProjectID
	case errors.Is(err, domain.ErrInvalidGitLabProjectPath):
		return registry.ErrInvalidGitLabProjectPath
	default:
		return err
	}
}

func (fake *fakeRegistry) PublishModuleVersion(ctx context.Context, req registry.PublishModuleVersionRequest) (registry.PublishModuleVersionResponse, error) {
	if fake.publishErr != nil {
		return registry.PublishModuleVersionResponse{}, fake.publishErr
	}
	module, exists := fake.modules[req.ModuleName]
	if !exists {
		return registry.PublishModuleVersionResponse{}, registry.ErrModuleNotFound
	}
	versionValue, err := domain.NewVersion(req.Version)
	if err != nil {
		return registry.PublishModuleVersionResponse{}, registry.ErrInvalidVersion
	}
	key := req.ModuleName + ":" + req.Version
	if _, exists := fake.versions[key]; exists {
		return registry.PublishModuleVersionResponse{}, registry.ErrModuleVersionAlreadyExists
	}
	body, err := io.ReadAll(req.Artifact)
	if err != nil {
		return registry.PublishModuleVersionResponse{}, err
	}
	sum := sha256.Sum256(body)
	checksum := hex.EncodeToString(sum[:])
	publishedAt := fake.now
	moduleVersion := domain.ModuleVersion{
		ID:          domain.NewModuleVersionID("version-" + req.Version),
		ModuleID:    module.ID,
		Version:     versionValue,
		Digest:      "sha256:" + checksum,
		Status:      domain.ModuleVersionStatusPublished,
		CreatedAt:   fake.now,
		PublishedAt: &publishedAt,
	}
	sourceArtifact := domain.Artifact{
		ID:              domain.NewArtifactID("artifact-" + req.Version),
		ModuleVersionID: moduleVersion.ID,
		Kind:            domain.ArtifactKindSourceArchive,
		StorageKey:      "artifact-key-" + req.Version,
		ChecksumSHA256:  checksum,
		SizeBytes:       int64(len(body)),
		CreatedAt:       fake.now,
	}
	bufImageArtifact := domain.Artifact{
		ID:              domain.NewArtifactID("buf-image-" + req.Version),
		ModuleVersionID: moduleVersion.ID,
		Kind:            domain.ArtifactKindBufImage,
		StorageKey:      "buf-image-key-" + req.Version,
		ChecksumSHA256:  "buf-image-checksum",
		SizeBytes:       int64(len("buf-image")),
		CreatedAt:       fake.now,
	}
	bufConfig := domain.BufConfigInfo{
		BufYAMLPresent:        true,
		BufLockPresent:        true,
		BufYAMLDigest:         "buf-yaml-digest",
		BufLockDigest:         "buf-lock-digest",
		ModulePaths:           []string{"proto"},
		Deps:                  []string{"buf.build/googleapis/googleapis"},
		LintEnabled:           fake.publishLint.Status != domain.BufLintStatusNotRun,
		BreakingConfigPresent: true,
	}
	metadata := testDescriptorMetadata()
	fake.versions[key] = moduleVersion
	fake.artifacts[key] = []domain.Artifact{sourceArtifact, bufImageArtifact}
	fake.objects[key] = body
	fake.bufConfigs[key] = bufConfig
	fake.lintResults[key] = fake.publishLint
	fake.metadata[key] = metadata
	return registry.PublishModuleVersionResponse{
		Version:          moduleVersion,
		SourceArtifact:   sourceArtifact,
		BufImageArtifact: bufImageArtifact,
		BufConfig:        bufConfig,
		LintResult:       fake.publishLint,
		MetadataSummary:  metadata.Summary(),
	}, nil
}

func (fake *fakeRegistry) ListModuleVersions(ctx context.Context, moduleName string, limit int, offset int) ([]domain.ModuleVersion, error) {
	if _, exists := fake.modules[moduleName]; !exists {
		return nil, registry.ErrModuleNotFound
	}
	versions := make([]domain.ModuleVersion, 0)
	for key, version := range fake.versions {
		if strings.HasPrefix(key, moduleName+":") {
			versions = append(versions, version)
		}
	}
	return versions, nil
}

func (fake *fakeRegistry) GetModuleVersion(ctx context.Context, moduleName string, version string) (domain.ModuleVersion, error) {
	if _, exists := fake.modules[moduleName]; !exists {
		return domain.ModuleVersion{}, registry.ErrModuleNotFound
	}
	moduleVersion, exists := fake.versions[moduleName+":"+version]
	if !exists {
		return domain.ModuleVersion{}, registry.ErrModuleNotFound
	}
	return moduleVersion, nil
}

func (fake *fakeRegistry) GetModuleVersionDetails(ctx context.Context, moduleName string, version string) (registry.ModuleVersionDetailsResponse, error) {
	moduleVersion, err := fake.GetModuleVersion(ctx, moduleName, version)
	if err != nil {
		return registry.ModuleVersionDetailsResponse{}, err
	}
	key := moduleName + ":" + version
	metadata := fake.metadata[key]
	return registry.ModuleVersionDetailsResponse{
		Version:         moduleVersion,
		Artifacts:       fake.artifacts[key],
		BufConfig:       fake.bufConfigs[key],
		LintResult:      fake.lintResults[key],
		MetadataSummary: metadata.Summary(),
	}, nil
}

func (fake *fakeRegistry) GetModuleVersionMetadata(ctx context.Context, moduleName string, version string) (domain.DescriptorMetadata, error) {
	if _, err := fake.GetModuleVersion(ctx, moduleName, version); err != nil {
		return domain.DescriptorMetadata{}, err
	}
	metadata, exists := fake.metadata[moduleName+":"+version]
	if !exists {
		return domain.DescriptorMetadata{}, registry.ErrModuleNotFound
	}
	return metadata, nil
}

func (fake *fakeRegistry) CheckBreaking(ctx context.Context, req registry.CheckBreakingRequest) (registry.CheckBreakingResponse, error) {
	if fake.checkErr != nil {
		return registry.CheckBreakingResponse{}, fake.checkErr
	}
	module, exists := fake.modules[req.ModuleName]
	if !exists {
		return registry.CheckBreakingResponse{}, registry.ErrModuleNotFound
	}
	against := req.Against
	if against == "latest" {
		var latest domain.ModuleVersion
		for key, version := range fake.versions {
			if strings.HasPrefix(key, req.ModuleName+":") && (latest.ID == "" || version.CreatedAt.After(latest.CreatedAt)) {
				latest = version
			}
		}
		if latest.ID == "" {
			return registry.CheckBreakingResponse{}, registry.ErrBaselineVersionNotFound
		}
		against = latest.Version.String()
	}
	baseline, exists := fake.versions[req.ModuleName+":"+against]
	if !exists {
		return registry.CheckBreakingResponse{}, registry.ErrBaselineVersionNotFound
	}
	targetRef := req.TargetRef
	if targetRef == "" {
		targetRef = "local"
	}
	changes := make([]domain.BreakingChange, 0, len(fake.breakingChanges))
	reportID := domain.NewBreakingReportID(fmt.Sprintf("report-%d", len(fake.reports)+1))
	for index, change := range fake.breakingChanges {
		change.ID = domain.NewBreakingChangeID(fmt.Sprintf("change-%d", index+1))
		change.ReportID = reportID
		change.CreatedAt = fake.now
		changes = append(changes, change)
	}
	report := domain.BreakingReport{
		ID:            reportID,
		ModuleID:      module.ID,
		ModuleName:    module.Name,
		BaseVersionID: baseline.ID,
		BaseVersion:   baseline.Version,
		TargetRef:     targetRef,
		Status:        fake.breakingStatus,
		ChangeCount:   len(changes),
		HumanSummary:  "ProtoRadar Breaking Change Report\nStatus: " + fake.breakingStatus.String(),
		CreatedAt:     fake.now,
	}
	response := registry.CheckBreakingResponse{Report: report, Changes: changes}
	fake.reports[report.ID.String()] = response
	return response, nil
}

func (fake *fakeRegistry) GetBreakingReport(ctx context.Context, reportID string) (registry.CheckBreakingResponse, error) {
	response, exists := fake.reports[reportID]
	if !exists {
		return registry.CheckBreakingResponse{}, registry.ErrBreakingReportNotFound
	}
	return response, nil
}

func (fake *fakeRegistry) ListBreakingReports(ctx context.Context, moduleName string, limit int, offset int) ([]domain.BreakingReport, error) {
	module, exists := fake.modules[moduleName]
	if !exists {
		return nil, registry.ErrModuleNotFound
	}
	reports := make([]domain.BreakingReport, 0)
	for _, response := range fake.reports {
		if response.Report.ModuleID == module.ID {
			reports = append(reports, response.Report)
		}
	}
	slices.SortFunc(reports, func(left domain.BreakingReport, right domain.BreakingReport) int {
		if left.CreatedAt.After(right.CreatedAt) {
			return -1
		}
		if left.CreatedAt.Before(right.CreatedAt) {
			return 1
		}
		return strings.Compare(left.ID.String(), right.ID.String())
	})
	if limit > 0 && len(reports) > limit {
		reports = reports[:limit]
	}
	return reports, nil
}

func (fake *fakeRegistry) GetModuleDependencyGraph(ctx context.Context, moduleName string) (registry.ModuleDependencyGraphResponse, error) {
	if fake.dependencyErr != nil {
		return registry.ModuleDependencyGraphResponse{}, fake.dependencyErr
	}
	module, exists := fake.modules[moduleName]
	if !exists {
		return registry.ModuleDependencyGraphResponse{}, registry.ErrModuleNotFound
	}
	upstream := make([]domain.ModuleDependency, 0)
	downstream := make([]domain.ModuleDependency, 0)
	for _, dependency := range fake.dependencies {
		if dependency.ConsumerModuleID == module.ID {
			upstream = append(upstream, dependency)
		}
		if dependency.ProviderModuleID == module.ID {
			downstream = append(downstream, dependency)
		}
	}
	unresolved := make([]domain.UnresolvedProtoDependency, 0)
	for _, dependency := range fake.unresolved {
		if dependency.ModuleID == module.ID {
			unresolved = append(unresolved, dependency)
		}
	}
	return registry.ModuleDependencyGraphResponse{Module: module, Upstream: upstream, Downstream: downstream, Unresolved: unresolved}, nil
}

func (fake *fakeRegistry) ListAffectedModules(ctx context.Context, moduleName string) (registry.AffectedModulesResponse, error) {
	if fake.dependencyErr != nil {
		return registry.AffectedModulesResponse{}, fake.dependencyErr
	}
	module, exists := fake.modules[moduleName]
	if !exists {
		return registry.AffectedModulesResponse{}, registry.ErrModuleNotFound
	}
	return registry.AffectedModulesResponse{Module: module, AffectedModules: fake.affectedModules(module.ID)}, nil
}

func (fake *fakeRegistry) GetBreakingReportAffectedModules(ctx context.Context, reportID string) (registry.BreakingReportAffectedModulesResponse, error) {
	if fake.dependencyErr != nil {
		return registry.BreakingReportAffectedModulesResponse{}, fake.dependencyErr
	}
	response, exists := fake.reports[reportID]
	if !exists {
		return registry.BreakingReportAffectedModulesResponse{}, registry.ErrBreakingReportNotFound
	}
	return registry.BreakingReportAffectedModulesResponse{
		Report:          response.Report,
		AffectedModules: fake.affectedModules(response.Report.ModuleID),
	}, nil
}

func (fake *fakeRegistry) affectedModules(providerModuleID domain.ModuleID) []domain.AffectedModule {
	type item struct {
		name    domain.ModuleName
		version domain.Version
		sources map[domain.DependencySource]struct{}
		reasons map[domain.DependencyResolutionReason]struct{}
	}
	groups := map[string]*item{}
	for _, dependency := range fake.dependencies {
		if dependency.ProviderModuleID != providerModuleID {
			continue
		}
		key := dependency.ConsumerModuleID.String()
		group, exists := groups[key]
		if !exists {
			group = &item{
				name:    dependency.ConsumerModuleName,
				version: dependency.ConsumerVersion,
				sources: map[domain.DependencySource]struct{}{},
				reasons: map[domain.DependencyResolutionReason]struct{}{},
			}
			groups[key] = group
		}
		group.sources[dependency.Source] = struct{}{}
		group.reasons[dependency.Reason] = struct{}{}
	}
	items := make([]domain.AffectedModule, 0, len(groups))
	for _, group := range groups {
		sources := make([]domain.DependencySource, 0, len(group.sources))
		for source := range group.sources {
			sources = append(sources, source)
		}
		reasons := make([]domain.DependencyResolutionReason, 0, len(group.reasons))
		for reason := range group.reasons {
			reasons = append(reasons, reason)
		}
		items = append(items, domain.AffectedModule{
			ModuleName:        group.name,
			LatestVersion:     group.version,
			DependencySources: sources,
			Reasons:           reasons,
		})
	}
	return items
}

func (fake *fakeRegistry) DownloadArtifact(ctx context.Context, moduleName string, version string) (storage.ArtifactObject, domain.Artifact, error) {
	key := moduleName + ":" + version
	artifacts, exists := fake.artifacts[key]
	if !exists || len(artifacts) == 0 {
		return storage.ArtifactObject{}, domain.Artifact{}, registry.ErrModuleNotFound
	}
	artifact := artifacts[0]
	body := fake.objects[key]
	return storage.ArtifactObject{
		Key:         artifact.StorageKey,
		ContentType: "application/gzip",
		SizeBytes:   int64(len(body)),
		Body:        io.NopCloser(bytes.NewReader(body)),
	}, artifact, nil
}

func (fake *fakeRegistry) ReportRuntimeInventory(ctx context.Context, input runtimeinventory.ReportRuntimeInventoryInput) (runtimeinventory.ReportRuntimeInventoryOutput, error) {
	if fake.runtimeErr != nil {
		return runtimeinventory.ReportRuntimeInventoryOutput{}, fake.runtimeErr
	}
	serviceName, err := domain.NewRuntimeServiceName(input.ServiceName)
	if err != nil {
		return runtimeinventory.ReportRuntimeInventoryOutput{}, err
	}
	environment, err := domain.NewRuntimeEnvironment(input.Environment)
	if err != nil {
		return runtimeinventory.ReportRuntimeInventoryOutput{}, err
	}
	if err := domain.ValidateRuntimeGitCommit(input.GitCommit); err != nil {
		return runtimeinventory.ReportRuntimeInventoryOutput{}, err
	}
	if err := domain.ValidateRuntimeBuildVersion(input.BuildVersion); err != nil {
		return runtimeinventory.ReportRuntimeInventoryOutput{}, err
	}
	if len(input.Modules) == 0 {
		return runtimeinventory.ReportRuntimeInventoryOutput{}, runtimeinventory.ErrRuntimeModulesRequired
	}
	service := fake.runtimeServices[serviceName.String()]
	if service.ID == "" {
		service = domain.RuntimeService{
			ID:        domain.NewRuntimeServiceID("runtime-service-" + serviceName.String()),
			Name:      serviceName,
			CreatedAt: fake.now,
			UpdatedAt: fake.now,
		}
		fake.runtimeServices[serviceName.String()] = service
	}
	deployment := domain.RuntimeDeployment{
		ID:           domain.NewRuntimeDeploymentID(fmt.Sprintf("runtime-deployment-%d", len(fake.runtimeDeployments)+1)),
		ServiceID:    service.ID,
		ServiceName:  service.Name,
		Environment:  environment,
		GitCommit:    input.GitCommit,
		BuildVersion: input.BuildVersion,
		ReportedAt:   fake.now,
		CreatedAt:    fake.now,
	}
	fake.runtimeDeployments = append(fake.runtimeDeployments, deployment)

	seen := map[string]struct{}{}
	outputUsages := make([]runtimeinventory.RuntimeModuleUsageOutput, 0, len(input.Modules))
	for _, reported := range input.Modules {
		moduleName, err := domain.NewModuleName(reported.Module)
		if err != nil {
			return runtimeinventory.ReportRuntimeInventoryOutput{}, err
		}
		version, err := domain.NewVersion(reported.Version)
		if err != nil {
			return runtimeinventory.ReportRuntimeInventoryOutput{}, err
		}
		key := moduleName.String() + "\x00" + version.String()
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		usage, output := fake.runtimeUsageForReport(deployment.ID, moduleName, version)
		fake.runtimeUsages = append(fake.runtimeUsages, usage)
		outputUsages = append(outputUsages, output)
	}
	if len(outputUsages) == 0 {
		return runtimeinventory.ReportRuntimeInventoryOutput{}, runtimeinventory.ErrRuntimeModulesRequired
	}
	return runtimeinventory.ReportRuntimeInventoryOutput{
		DeploymentID: deployment.ID.String(),
		ServiceName:  serviceName.String(),
		Environment:  environment.String(),
		GitCommit:    input.GitCommit,
		BuildVersion: input.BuildVersion,
		ReportedAt:   fake.now,
		Usages:       outputUsages,
	}, nil
}

func (fake *fakeRegistry) runtimeUsageForReport(deploymentID domain.RuntimeDeploymentID, moduleName domain.ModuleName, version domain.Version) (domain.RuntimeModuleUsage, runtimeinventory.RuntimeModuleUsageOutput) {
	status := domain.RuntimeDriftStatusUnknownVersion
	reason := "module_not_found"
	latestVersion := ""
	var moduleID *domain.ModuleID
	var moduleVersionID *domain.ModuleVersionID
	if module, exists := fake.modules[moduleName.String()]; exists {
		moduleID = &module.ID
		if reported, exists := fake.versions[moduleName.String()+":"+version.String()]; exists {
			moduleVersionID = &reported.ID
			latest := fake.latestVersion(moduleName.String())
			if latest.ID != "" {
				latestVersion = latest.Version.String()
			}
			if latest.ID == reported.ID {
				status = domain.RuntimeDriftStatusUpToDate
				reason = "up_to_date"
			} else {
				status = domain.RuntimeDriftStatusBehindLatest
				reason = "behind_latest: latest version is " + latestVersion
			}
		} else {
			reason = "version_not_found"
		}
	}
	usage := domain.RuntimeModuleUsage{
		ID:              domain.NewRuntimeModuleUsageID(fmt.Sprintf("runtime-usage-%d", len(fake.runtimeUsages)+1)),
		DeploymentID:    deploymentID,
		ModuleID:        moduleID,
		ModuleName:      moduleName,
		ModuleVersionID: moduleVersionID,
		Version:         version,
		DriftStatus:     status,
		DriftReason:     reason,
		CreatedAt:       fake.now,
	}
	if latestVersion != "" {
		latest, _ := domain.NewVersion(latestVersion)
		usage.LatestVersion = &latest
	}
	return usage, runtimeinventory.RuntimeModuleUsageOutput{
		Module:        moduleName.String(),
		Version:       version.String(),
		LatestVersion: latestVersion,
		DriftStatus:   status,
		DriftReason:   reason,
	}
}

func (fake *fakeRegistry) latestVersion(moduleName string) domain.ModuleVersion {
	var latest domain.ModuleVersion
	for key, version := range fake.versions {
		if strings.HasPrefix(key, moduleName+":") && (latest.ID == "" || version.CreatedAt.After(latest.CreatedAt)) {
			latest = version
		}
	}
	return latest
}

func (fake *fakeRegistry) ListRuntimeServices(ctx context.Context, limit int, offset int) ([]domain.RuntimeServiceSummary, error) {
	if fake.runtimeErr != nil {
		return nil, fake.runtimeErr
	}
	summaries := make([]domain.RuntimeServiceSummary, 0, len(fake.runtimeServices))
	for _, service := range fake.runtimeServices {
		summary := domain.RuntimeServiceSummary{Service: service}
		environmentSet := map[string]domain.RuntimeEnvironment{}
		for _, deployment := range fake.runtimeDeployments {
			if deployment.ServiceID != service.ID {
				continue
			}
			summary.DeploymentCount++
			environmentSet[deployment.Environment.String()] = deployment.Environment
			if summary.LatestReportedAt == nil || deployment.ReportedAt.After(*summary.LatestReportedAt) {
				value := deployment.ReportedAt
				summary.LatestReportedAt = &value
			}
			for _, usage := range fake.runtimeUsages {
				if usage.DeploymentID == deployment.ID {
					addRuntimeSummaryDrift(&summary, usage.DriftStatus)
				}
			}
		}
		for _, environment := range environmentSet {
			summary.Environments = append(summary.Environments, environment)
		}
		summary.EnvironmentCount = len(summary.Environments)
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func addRuntimeSummaryDrift(summary *domain.RuntimeServiceSummary, status domain.RuntimeDriftStatus) {
	switch status {
	case domain.RuntimeDriftStatusUpToDate:
		summary.UpToDateCount++
	case domain.RuntimeDriftStatusBehindLatest:
		summary.BehindLatestCount++
	case domain.RuntimeDriftStatusUnknownVersion:
		summary.UnknownVersionCount++
	case domain.RuntimeDriftStatusDeprecatedVersion:
		summary.DeprecatedCount++
	}
}

func (fake *fakeRegistry) GetRuntimeServiceDetails(ctx context.Context, serviceNameValue string) (domain.RuntimeServiceDetails, error) {
	if fake.runtimeErr != nil {
		return domain.RuntimeServiceDetails{}, fake.runtimeErr
	}
	service, exists := fake.runtimeServices[serviceNameValue]
	if !exists {
		return domain.RuntimeServiceDetails{}, domain.ErrNotFound
	}
	deployments := make([]domain.RuntimeDeployment, 0)
	usages := make([]domain.RuntimeModuleUsage, 0)
	for _, deployment := range fake.runtimeDeployments {
		if deployment.ServiceID != service.ID {
			continue
		}
		deployments = append(deployments, deployment)
		for _, usage := range fake.runtimeUsages {
			if usage.DeploymentID == deployment.ID {
				usages = append(usages, usage)
			}
		}
	}
	return domain.RuntimeServiceDetails{Service: service, Deployments: deployments, Usages: usages}, nil
}

func (fake *fakeRegistry) GetEnvironmentInventory(ctx context.Context, environmentValue string, limit int, offset int) (domain.RuntimeEnvironmentInventory, error) {
	if fake.runtimeErr != nil {
		return domain.RuntimeEnvironmentInventory{}, fake.runtimeErr
	}
	environment, err := domain.NewRuntimeEnvironment(environmentValue)
	if err != nil {
		return domain.RuntimeEnvironmentInventory{}, err
	}
	deployments := make([]domain.RuntimeDeployment, 0)
	usages := make([]domain.RuntimeModuleUsage, 0)
	for _, deployment := range fake.runtimeDeployments {
		if deployment.Environment != environment {
			continue
		}
		deployments = append(deployments, deployment)
		for _, usage := range fake.runtimeUsages {
			if usage.DeploymentID == deployment.ID {
				usages = append(usages, usage)
			}
		}
	}
	return domain.RuntimeEnvironmentInventory{Environment: environment, Deployments: deployments, Usages: usages}, nil
}

func (fake *fakeRegistry) GetModuleRuntimeUsages(ctx context.Context, moduleNameValue string, limit int, offset int) ([]domain.ModuleRuntimeUsage, error) {
	if fake.runtimeErr != nil {
		return nil, fake.runtimeErr
	}
	moduleName, err := domain.NewModuleName(moduleNameValue)
	if err != nil {
		return nil, err
	}
	items := make([]domain.ModuleRuntimeUsage, 0)
	for _, usage := range fake.runtimeUsages {
		if usage.ModuleName != moduleName {
			continue
		}
		for _, deployment := range fake.runtimeDeployments {
			if deployment.ID == usage.DeploymentID {
				items = append(items, domain.ModuleRuntimeUsage{
					ServiceName:  deployment.ServiceName,
					Environment:  deployment.Environment,
					DeploymentID: deployment.ID,
					ModuleName:   usage.ModuleName,
					Version:      usage.Version,
					GitCommit:    deployment.GitCommit,
					BuildVersion: deployment.BuildVersion,
					ReportedAt:   deployment.ReportedAt,
					DriftStatus:  usage.DriftStatus,
					DriftReason:  usage.DriftReason,
				})
			}
		}
	}
	return items, nil
}

func (fake *fakeRegistry) GetBreakingReportRuntimeImpact(ctx context.Context, reportID domain.BreakingReportID, limit int, offset int) ([]domain.RuntimeImpact, error) {
	if fake.runtimeErr != nil {
		return nil, fake.runtimeErr
	}
	report, exists := fake.reports[reportID.String()]
	if !exists {
		return nil, registry.ErrBreakingReportNotFound
	}
	items := make([]domain.RuntimeImpact, 0)
	for _, usage := range fake.runtimeUsages {
		if usage.ModuleVersionID == nil || *usage.ModuleVersionID != report.Report.BaseVersionID {
			continue
		}
		for _, deployment := range fake.runtimeDeployments {
			if deployment.ID == usage.DeploymentID {
				items = append(items, domain.RuntimeImpact{
					ServiceName:  deployment.ServiceName,
					Environment:  deployment.Environment,
					UsedModule:   usage.ModuleName,
					UsedVersion:  usage.Version,
					GitCommit:    deployment.GitCommit,
					BuildVersion: deployment.BuildVersion,
					ReportedAt:   deployment.ReportedAt,
					ImpactStatus: domain.RuntimeImpactStatusPotentiallyAffectedByBreakingChange,
					Reason:       "exact runtime module version matches breaking report base version",
				})
			}
		}
	}
	return items, nil
}

func (fake *fakeRegistry) seedRuntimeServiceSummary(t *testing.T, serviceNameValue string, environmentValue string) {
	t.Helper()
	serviceName, _ := domain.NewRuntimeServiceName(serviceNameValue)
	environment, _ := domain.NewRuntimeEnvironment(environmentValue)
	fake.runtimeServices[serviceName.String()] = domain.RuntimeService{
		ID:        domain.NewRuntimeServiceID("runtime-service-" + serviceName.String()),
		Name:      serviceName,
		CreatedAt: fake.now,
		UpdatedAt: fake.now,
	}
	fake.runtimeDeployments = append(fake.runtimeDeployments, domain.RuntimeDeployment{
		ID:           domain.NewRuntimeDeploymentID("runtime-deployment-summary"),
		ServiceID:    fake.runtimeServices[serviceName.String()].ID,
		ServiceName:  serviceName,
		Environment:  environment,
		GitCommit:    "abc123",
		BuildVersion: "build-1",
		ReportedAt:   fake.now,
		CreatedAt:    fake.now,
	})
}

func (fake *fakeRegistry) seedRuntimeDeployment(t *testing.T, serviceNameValue string, environmentValue string, moduleNameValue string, versionValue string) {
	t.Helper()
	serviceName, _ := domain.NewRuntimeServiceName(serviceNameValue)
	environment, _ := domain.NewRuntimeEnvironment(environmentValue)
	moduleName, _ := domain.NewModuleName(moduleNameValue)
	version, _ := domain.NewVersion(versionValue)
	service := fake.runtimeServices[serviceName.String()]
	if service.ID == "" {
		service = domain.RuntimeService{
			ID:        domain.NewRuntimeServiceID("runtime-service-" + serviceName.String()),
			Name:      serviceName,
			CreatedAt: fake.now,
			UpdatedAt: fake.now,
		}
		fake.runtimeServices[serviceName.String()] = service
	}
	deployment := domain.RuntimeDeployment{
		ID:           domain.NewRuntimeDeploymentID(fmt.Sprintf("runtime-deployment-%d", len(fake.runtimeDeployments)+1)),
		ServiceID:    service.ID,
		ServiceName:  serviceName,
		Environment:  environment,
		GitCommit:    "abc123",
		BuildVersion: "build-1",
		ReportedAt:   fake.now,
		CreatedAt:    fake.now,
	}
	fake.runtimeDeployments = append(fake.runtimeDeployments, deployment)
	usage, _ := fake.runtimeUsageForReport(deployment.ID, moduleName, version)
	fake.runtimeUsages = append(fake.runtimeUsages, usage)
}

func testDescriptorMetadata() domain.DescriptorMetadata {
	return domain.DescriptorMetadata{Files: []domain.ProtoFile{
		{
			Path:        "user/v1/user.proto",
			PackageName: "user.v1",
			Syntax:      "proto3",
			Imports: []domain.ProtoImport{
				{Path: "google/protobuf/timestamp.proto", Public: true},
			},
			Services: []domain.ProtoService{
				{
					Name:     "UserService",
					FullName: "user.v1.UserService",
					Methods: []domain.ProtoMethod{
						{Name: "GetUser", InputType: ".user.v1.GetUserRequest", OutputType: ".user.v1.User"},
					},
				},
			},
			Messages: []domain.ProtoMessage{
				{
					Name:     "User",
					FullName: "user.v1.User",
					Fields: []domain.ProtoField{
						{Name: "id", Number: 1, Type: "string", JSONName: "id"},
					},
				},
			},
			Enums: []domain.ProtoEnum{
				{
					Name:     "Role",
					FullName: "user.v1.Role",
					Values: []domain.ProtoEnumValue{
						{Name: "ROLE_UNSPECIFIED", Number: 0},
					},
				},
			},
		},
	}}
}

func (fake *fakeRegistry) CreateAPIToken(ctx context.Context, req registry.CreateAPITokenRequest) (registry.CreateAPITokenResponse, error) {
	token := domain.APIToken{
		ID:        domain.NewAPITokenID("token-1"),
		Name:      req.Name,
		TokenHash: "stored-hash",
		CreatedAt: fake.now,
		ExpiresAt: req.ExpiresAt,
	}
	return registry.CreateAPITokenResponse{
		Token:    token,
		RawToken: "raw-token",
	}, nil
}
