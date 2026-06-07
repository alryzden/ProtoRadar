package bootstrap

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/config"
	"github.com/alryzden/ProtoRadar/internal/outbox"
	httptransport "github.com/alryzden/ProtoRadar/internal/transport/http"
	"github.com/alryzden/ProtoRadar/internal/usecase/uiquery"
)

func TestNewHTTPHandlerRegistersUIWhenEnabled(t *testing.T) {
	cfg := runtimeConfigForHTTPTest()
	cfg.UI.Enabled = true

	handler, err := NewHTTPHandler(nil, nil, nil, nil, nil, nil, fakeUIQuery{}, cfg, func(ctx context.Context) error { return nil })
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

	handler, err := NewHTTPHandler(nil, nil, nil, nil, nil, nil, nil, cfg, func(ctx context.Context) error { return nil })
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	res := request(t, handler, "/ui/modules")
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

func TestNewHTTPServerUsesConfiguredTimeouts(t *testing.T) {
	cfg := runtimeConfigForHTTPTest()
	cfg.Server = config.RuntimeServerConfig{
		HTTPAddr:              ":9090",
		HTTPReadHeaderTimeout: 6 * time.Second,
		HTTPReadTimeout:       31 * time.Second,
		HTTPWriteTimeout:      61 * time.Second,
		HTTPIdleTimeout:       121 * time.Second,
		HTTPMaxHeaderBytes:    4096,
	}
	handler := noopHTTPHandler{}

	server := newHTTPServer(cfg, handler)

	if server.Addr != ":9090" {
		t.Fatalf("addr = %q", server.Addr)
	}
	if server.Handler != handler {
		t.Fatalf("handler was not wired")
	}
	if server.ReadHeaderTimeout != 6*time.Second {
		t.Fatalf("read header timeout = %s", server.ReadHeaderTimeout)
	}
	if server.ReadTimeout != 31*time.Second {
		t.Fatalf("read timeout = %s", server.ReadTimeout)
	}
	if server.WriteTimeout != 61*time.Second {
		t.Fatalf("write timeout = %s", server.WriteTimeout)
	}
	if server.IdleTimeout != 121*time.Second {
		t.Fatalf("idle timeout = %s", server.IdleTimeout)
	}
	if server.MaxHeaderBytes != 4096 {
		t.Fatalf("max header bytes = %d", server.MaxHeaderBytes)
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

type noopHTTPHandler struct{}

func (noopHTTPHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

func (fakeUIQuery) GetEdition(ctx context.Context, input uiquery.GetEditionInput) (uiquery.EditionDetails, error) {
	return uiquery.EditionDetails{}, nil
}

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

func (fakeUIQuery) GetApprovalRequestDetails(ctx context.Context, input uiquery.GetApprovalRequestDetailsInput) (uiquery.ApprovalRequestDetails, error) {
	return uiquery.ApprovalRequestDetails{}, nil
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

func TestStartOutboxPublisherRunsAndStops(t *testing.T) {
	logger, err := httptransport.NewLogger("info", "json", io.Discard)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	metrics := httptransport.NewMetrics()
	repo := &fakeOutboxRepository{claimed: make(chan struct{})}
	cfg := runtimeConfigForHTTPTest()
	cfg.OutboxPublisher = config.RuntimeOutboxPublisherConfig{
		Enabled:             true,
		BatchSize:           2,
		PollInterval:        time.Hour,
		LeaseDuration:       time.Minute,
		MaxAttempts:         3,
		InitialRetryBackoff: time.Second,
		MaxRetryBackoff:     time.Minute,
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	cancel, done, err := startOutboxPublisher(ctx, cfg, repo, logger, metrics)
	if err != nil {
		t.Fatalf("start publisher: %v", err)
	}
	select {
	case <-repo.claimed:
	case <-time.After(time.Second):
		t.Fatalf("publisher did not claim on startup")
	}
	cancel()
	cancelCtx()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("publisher did not stop")
	}
}

func TestWaitOutboxShutdownReturnsWhenDoneCloses(t *testing.T) {
	done := make(chan struct{})
	close(done)

	if err := waitOutboxShutdown(done, time.Second); err != nil {
		t.Fatalf("wait outbox shutdown: %v", err)
	}
}

func TestWaitOutboxShutdownTimesOutWhenDoneNeverCloses(t *testing.T) {
	done := make(chan struct{})

	started := time.Now()
	err := waitOutboxShutdown(done, 10*time.Millisecond)
	if err == nil {
		t.Fatalf("expected timeout")
	}
	if time.Since(started) > time.Second {
		t.Fatalf("timeout wait took too long")
	}
}

type fakeOutboxRepository struct {
	claimCalls int
	claimed    chan struct{}
}

func (repo *fakeOutboxRepository) Create(context.Context, outbox.Record) error {
	return nil
}

func (repo *fakeOutboxRepository) ClaimPending(context.Context, int, time.Duration, time.Time) ([]outbox.Record, error) {
	repo.claimCalls++
	if repo.claimCalls == 1 && repo.claimed != nil {
		close(repo.claimed)
	}
	return nil, nil
}

func (repo *fakeOutboxRepository) MarkPublished(context.Context, string, time.Time) error {
	return nil
}

func (repo *fakeOutboxRepository) MarkFailed(context.Context, string, string, time.Time, time.Time, int) error {
	return nil
}

func (repo *fakeOutboxRepository) Stats(context.Context) (outbox.Stats, error) {
	return outbox.Stats{Pending: 1, Processing: 2, Failed: 3, Dead: 4}, nil
}
