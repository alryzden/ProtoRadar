package httptransport

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
)

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

func TestPostRuntimeReportDeprecatedVersionReturnsDriftStatus(t *testing.T) {
	fake := newFakeRegistry()
	server := newRuntimeTestServerWithFake(fake)
	user := createHTTPModule(t, server, "user-api")
	version := fake.addVersion(t, user, "v1.2.0")
	deprecatedAt := fake.now.Add(time.Minute)
	version.DeprecatedAt = &deprecatedAt
	version.DeprecatedBy = "maintainer"
	version.DeprecationReason = "Use v1.5.0 instead."
	fake.versions["user-api:v1.2.0"] = version
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
	if len(body.Usages) != 1 || body.Usages[0].DriftStatus != "deprecated_version" || body.Usages[0].DriftReason != "deprecated_version" || body.Usages[0].LatestVersion != "v1.5.0" {
		t.Fatalf("usages = %#v", body.Usages)
	}
}

func TestRuntimeDeprecatedVersionPropagatesToReadEndpoints(t *testing.T) {
	fake := newFakeRegistry()
	server := newRuntimeTestServerWithFake(fake)
	user := createHTTPModule(t, server, "user-api")
	version := fake.addVersion(t, user, "v1.2.0")
	deprecatedAt := fake.now.Add(time.Minute)
	version.DeprecatedAt = &deprecatedAt
	version.DeprecatedBy = "maintainer"
	fake.versions["user-api:v1.2.0"] = version

	res := request(t, server, http.MethodPost, "/api/v1/runtime/reports", strings.NewReader(`{
		"service_name": "billing-service",
		"environment": "production",
		"git_commit": "abc1234",
		"build_version": "build-1",
		"modules": [{"module": "user-api", "version": "v1.2.0"}]
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusCreated {
		t.Fatalf("report status = %d, body = %s", res.Code, res.Body.String())
	}

	servicesRes := request(t, server, http.MethodGet, "/api/v1/runtime/services", nil, "Bearer valid", "")
	var services listRuntimeServicesResponse
	decodeResponse(t, servicesRes, &services)
	if len(services.Services) != 1 || services.Services[0].DriftSummary.DeprecatedVersion != 1 {
		t.Fatalf("services = %#v", services)
	}

	serviceRes := request(t, server, http.MethodGet, "/api/v1/runtime/services/billing-service", nil, "Bearer valid", "")
	var serviceDetails runtimeServiceDetailsDTO
	decodeResponse(t, serviceRes, &serviceDetails)
	if len(serviceDetails.Usages) != 1 || serviceDetails.Usages[0].DriftStatus != "deprecated_version" {
		t.Fatalf("service details = %#v", serviceDetails)
	}

	environmentRes := request(t, server, http.MethodGet, "/api/v1/runtime/environments/production", nil, "Bearer valid", "")
	var environment runtimeEnvironmentInventoryDTO
	decodeResponse(t, environmentRes, &environment)
	if len(environment.Usages) != 1 || environment.Usages[0].DriftStatus != "deprecated_version" {
		t.Fatalf("environment = %#v", environment)
	}

	moduleRes := request(t, server, http.MethodGet, "/api/v1/modules/user-api/runtime-usages", nil, "Bearer valid", "")
	var moduleUsages moduleRuntimeUsagesDTO
	decodeResponse(t, moduleRes, &moduleUsages)
	if len(moduleUsages.Usages) != 1 || moduleUsages.Usages[0].DriftStatus != "deprecated_version" {
		t.Fatalf("module usages = %#v", moduleUsages)
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
	if body.Impacts[0].DriftStatus != "up_to_date" || body.Impacts[0].DriftReason != "up_to_date" {
		t.Fatalf("runtime impact drift = %#v", body.Impacts[0])
	}
}

func TestGetBreakingReportRuntimeImpactIncludesDeprecatedRuntimeDrift(t *testing.T) {
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
	fake.runtimeUsages[0].DriftStatus = domain.RuntimeDriftStatusDeprecatedVersion
	fake.runtimeUsages[0].DriftReason = "deprecated_version"

	res := request(t, server, http.MethodGet, "/api/v1/breaking-reports/report-1/runtime-impact", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body breakingReportRuntimeImpactDTO
	decodeResponse(t, res, &body)
	if len(body.Impacts) != 1 || body.Impacts[0].DriftStatus != "deprecated_version" || body.Impacts[0].DriftReason != "deprecated_version" {
		t.Fatalf("body = %#v", body)
	}
	for _, forbidden := range []string{"prr_", "Bearer", "token", "deprecated_by"} {
		if strings.Contains(res.Body.String(), forbidden) {
			t.Fatalf("runtime impact response leaked %q: %s", forbidden, res.Body.String())
		}
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
