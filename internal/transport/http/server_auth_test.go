package httptransport

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/authorization"
	"github.com/alryzden/ProtoRadar/internal/edition"
	"github.com/alryzden/ProtoRadar/internal/identity"
	"github.com/alryzden/ProtoRadar/internal/version"
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

func TestGetEditionReturnsCommunityCapabilities(t *testing.T) {
	build := version.BuildInfo{Version: "v1.2.3", Commit: "abc123", BuildDate: "2026-06-05T12:00:00Z"}
	server := NewServer(newFakeRegistry(), Options{
		BootstrapToken:    "bootstrap",
		CapabilityChecker: edition.NewCommunityCapabilityChecker(),
		BuildInfo:         build,
	}).Handler()

	res := request(t, server, http.MethodGet, "/api/v1/edition", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body editionResponse
	decodeResponse(t, res, &body)
	if body.Edition != edition.NameCommunity || body.Version != build.Version || body.Commit != build.Commit || body.BuildDate != build.BuildDate {
		t.Fatalf("edition response = %#v", body)
	}
	capabilities := map[string]bool{}
	for _, status := range body.Capabilities {
		capabilities[status.Name] = status.Enabled
	}
	if !capabilities[edition.CapabilityRegistry.String()] {
		t.Fatalf("registry capability disabled: %#v", capabilities)
	}
	if capabilities[edition.CapabilityOIDCAuth.String()] {
		t.Fatalf("oidc capability enabled: %#v", capabilities)
	}
	for _, forbidden := range []string{"raw-token", "stored-hash", "Authorization", "Bearer", "token_hash"} {
		if strings.Contains(res.Body.String(), forbidden) {
			t.Fatalf("edition response leaked %q: %s", forbidden, res.Body.String())
		}
	}
}

func TestGetEditionRequiresAuthentication(t *testing.T) {
	server := newTestServer()

	res := request(t, server, http.MethodGet, "/api/v1/edition", nil, "", "")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
	assertAPIError(t, res, "unauthorized")
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

func TestOutboxMetricsEndpointReturnsCountersAndGauges(t *testing.T) {
	metrics := NewMetrics()
	metrics.RecordClaimed(3)
	metrics.RecordDispatch("published", 2*time.Second)
	metrics.RecordDispatch("failed", time.Second)
	metrics.RecordError("claim_failed")
	metrics.SetOutboxStats(5, 1, 2, 3)
	server := NewServer(newFakeRegistry(), Options{
		BootstrapToken: "bootstrap",
		Metrics:        metrics,
	}).Handler()

	body := request(t, server, http.MethodGet, "/metrics", nil, "", "").Body.String()
	for _, want := range []string{
		"protoradar_outbox_claimed_total 3",
		`protoradar_outbox_dispatch_total{status="failed"} 1`,
		`protoradar_outbox_dispatch_total{status="published"} 1`,
		`protoradar_outbox_errors_total{status="claim_failed"} 1`,
		"protoradar_outbox_dispatch_duration_seconds_sum 3",
		"protoradar_outbox_pending_total 5",
		"protoradar_outbox_processing_total 1",
		"protoradar_outbox_failed_total 2",
		"protoradar_outbox_dead_total 3",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics missing %q:\n%s", want, body)
		}
	}
	for _, forbidden := range []string{"record_id", "dedup", "module_name", "payload"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("metrics contain high-cardinality or payload label %q:\n%s", forbidden, body)
		}
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

func TestRequireBearerStoresPrincipalInContext(t *testing.T) {
	auth := &fakeAuthProvider{
		principal: identity.Principal{
			Subject:     "ci",
			DisplayName: "ci",
			Type:        identity.PrincipalTypeAPIToken,
		},
	}
	server := NewServer(newFakeRegistry(), Options{AuthProvider: auth})
	handler := server.requireBearer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := identity.PrincipalFromContext(r.Context())
		if !ok {
			t.Fatalf("principal missing from context")
		}
		if principal.Subject != "ci" || principal.Type != identity.PrincipalTypeAPIToken {
			t.Fatalf("principal = %#v", principal)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	res := request(t, handler, http.MethodGet, "/protected", nil, "Bearer valid", "")
	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
	if auth.seen.AuthorizationHeader != "Bearer valid" {
		t.Fatalf("auth request header = %q", auth.seen.AuthorizationHeader)
	}
}

func TestRequireAuthorizationMissingPrincipalReturnsUnauthorized(t *testing.T) {
	server := NewServer(newFakeRegistry(), Options{})
	handler := server.requireAuthorization(authorization.ActionModuleRead, staticResource("module_collection"), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("handler should not be called")
	}))

	res := request(t, handler, http.MethodGet, "/protected", nil, "", "")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
	assertAPIError(t, res, "unauthorized")
}

func TestAuthorizationDenyReturnsForbidden(t *testing.T) {
	authorizer := &fakeAuthorizer{denyAll: true}
	server := newProtectedRouteTestServer(authorizer)

	res := performRequestWithBearer(t, server, http.MethodGet, "/api/v1/modules", nil, "")
	assertForbiddenByAuthorizer(t, res)
	assertAuthorizerCall(t, authorizer, authorization.ActionModuleRead, authorization.Resource{Type: "module_collection"})
}

func TestAuthorizationAllowReachesProtectedHandler(t *testing.T) {
	authorizer := &fakeAuthorizer{}
	server := newProtectedRouteTestServer(authorizer)

	res := performRequestWithBearer(t, server, http.MethodGet, "/api/v1/modules", nil, "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusOK, res.Body.String())
	}
	assertAuthorizerCall(t, authorizer, authorization.ActionModuleRead, authorization.Resource{Type: "module_collection"})
}

func TestAuthorizationHarnessPublicEndpointDoesNotCallAuthorizer(t *testing.T) {
	authorizer := &fakeAuthorizer{denyAll: true}
	server := newProtectedRouteTestServer(authorizer)

	res := request(t, server, http.MethodGet, "/healthz", nil, "", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusOK, res.Body.String())
	}
	assertAuthorizerNotCalled(t, authorizer)
}

func TestAuthorizationHarnessMissingBearerFailsBeforeAuthorizer(t *testing.T) {
	authorizer := &fakeAuthorizer{}
	server := newProtectedRouteTestServer(authorizer)

	res := request(t, server, http.MethodGet, "/api/v1/modules", nil, "", "")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusUnauthorized, res.Body.String())
	}
	assertAPIError(t, res, "unauthorized")
	assertAuthorizerNotCalled(t, authorizer)
}

func TestPublicRoutesDoNotInvokeAuthorizer(t *testing.T) {
	authorizer := &fakeAuthorizer{denyAll: true}
	server := NewServer(newFakeRegistry(), Options{
		BootstrapToken: "bootstrap",
		Authorizer:     authorizer,
		Ready: func(ctx context.Context) error {
			return nil
		},
		Metrics: NewMetrics(),
	}).Handler()

	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/healthz"},
		{method: http.MethodGet, path: "/readyz"},
		{method: http.MethodGet, path: "/metrics"},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			res := request(t, server, tt.method, tt.path, nil, "", "")
			if res.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusOK, res.Body.String())
			}
		})
	}
	assertAuthorizerNotCalled(t, authorizer)
}

func TestBootstrapTokenRouteDoesNotInvokeAuthorizer(t *testing.T) {
	authorizer := &fakeAuthorizer{denyAll: true}
	server := NewServer(newFakeRegistry(), Options{
		BootstrapToken: "bootstrap",
		Authorizer:     authorizer,
	}).Handler()

	res := request(t, server, http.MethodPost, "/api/v1/tokens", strings.NewReader(`{"name":"ci"}`), "Bearer bootstrap", "application/json")
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusCreated, res.Body.String())
	}
	assertAuthorizerNotCalled(t, authorizer)

	normalBearer := request(t, server, http.MethodPost, "/api/v1/tokens", strings.NewReader(`{"name":"ci"}`), "Bearer valid", "application/json")
	if normalBearer.Code != http.StatusUnauthorized {
		t.Fatalf("normal bearer status = %d, want %d; body = %s", normalBearer.Code, http.StatusUnauthorized, normalBearer.Body.String())
	}
	assertAPIError(t, normalBearer, "unauthorized")
	assertAuthorizerNotCalled(t, authorizer)
}
