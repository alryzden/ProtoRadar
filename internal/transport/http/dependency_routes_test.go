package httptransport

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
)

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
