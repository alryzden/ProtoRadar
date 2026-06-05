package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alryzden/ProtoRadar/internal/config"
	"github.com/alryzden/ProtoRadar/internal/usecase/uiquery"
)

func TestNewHTTPHandlerRegistersUIWhenEnabled(t *testing.T) {
	cfg := runtimeConfigForHTTPTest()
	cfg.UI.Enabled = true

	handler, err := NewHTTPHandler(nil, nil, fakeUIQuery{}, cfg, func(ctx context.Context) error { return nil })
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	res := request(t, handler, "/ui/modules")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
}

func TestNewHTTPHandlerDoesNotRegisterUIWhenDisabled(t *testing.T) {
	cfg := runtimeConfigForHTTPTest()
	cfg.UI.Enabled = false

	handler, err := NewHTTPHandler(nil, nil, nil, cfg, func(ctx context.Context) error { return nil })
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	res := request(t, handler, "/ui/modules")
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

func runtimeConfigForHTTPTest() config.RuntimeConfig {
	return config.RuntimeConfig{
		Auth: config.RuntimeAuthConfig{
			BootstrapToken: "bootstrap",
		},
		UI: config.RuntimeUIConfig{
			Enabled:    true,
			BasePath:   "/ui",
			StaticPath: "/ui/static",
		},
	}
}

type fakeUIQuery struct{}

func (fakeUIQuery) ListModuleOverviews(ctx context.Context, input uiquery.ListModuleOverviewsInput) ([]uiquery.ModuleOverview, error) {
	return []uiquery.ModuleOverview{}, nil
}

func (fakeUIQuery) GetModuleOverview(ctx context.Context, input uiquery.GetModuleOverviewInput) (uiquery.ModuleOverview, error) {
	return uiquery.ModuleOverview{}, nil
}

func (fakeUIQuery) GetVersionOverview(ctx context.Context, input uiquery.GetVersionOverviewInput) (uiquery.VersionOverview, error) {
	return uiquery.VersionOverview{}, nil
}

func (fakeUIQuery) GetModuleDependencyGraph(ctx context.Context, input uiquery.GetModuleDependencyGraphInput) (uiquery.ModuleDependencyGraph, error) {
	return uiquery.ModuleDependencyGraph{}, nil
}

func (fakeUIQuery) ListBreakingReportOverviews(ctx context.Context, input uiquery.ListBreakingReportOverviewsInput) ([]uiquery.BreakingReportSummary, error) {
	return []uiquery.BreakingReportSummary{}, nil
}

func (fakeUIQuery) GetBreakingReportDetails(ctx context.Context, input uiquery.GetBreakingReportDetailsInput) (uiquery.BreakingReportDetails, error) {
	return uiquery.BreakingReportDetails{}, nil
}

func (fakeUIQuery) ListRuntimeServices(ctx context.Context, input uiquery.ListRuntimeServicesInput) ([]uiquery.RuntimeServiceSummary, error) {
	return []uiquery.RuntimeServiceSummary{}, nil
}

func (fakeUIQuery) GetRuntimeServiceDetails(ctx context.Context, input uiquery.GetRuntimeServiceDetailsInput) (uiquery.RuntimeServiceDetails, error) {
	return uiquery.RuntimeServiceDetails{}, nil
}

func (fakeUIQuery) GetRuntimeEnvironmentInventory(ctx context.Context, input uiquery.GetRuntimeEnvironmentInventoryInput) (uiquery.RuntimeEnvironmentInventory, error) {
	return uiquery.RuntimeEnvironmentInventory{}, nil
}

func (fakeUIQuery) GetModuleRuntimeUsages(ctx context.Context, input uiquery.GetModuleRuntimeUsagesInput) (uiquery.ModuleRuntimeUsages, error) {
	return uiquery.ModuleRuntimeUsages{}, nil
}

func request(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}
