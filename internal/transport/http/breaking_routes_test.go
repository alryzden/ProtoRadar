package httptransport

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
)

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
