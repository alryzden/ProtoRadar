package httptransport

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
)

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

func TestDeprecateModuleVersionUsesAuthenticatedPrincipal(t *testing.T) {
	fake := newFakeRegistry()
	server := NewServer(fake, Options{BootstrapToken: "bootstrap"}).Handler()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")
	multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))

	res := request(t, server, http.MethodPost, "/api/v1/modules/user-api/versions/v1.0.0/deprecate", strings.NewReader(`{"reason":"Use v1.1.0 instead."}`), "Bearer valid", "application/json")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}

	if fake.deprecateInput.Actor != "test" {
		t.Fatalf("actor = %q, want authenticated principal", fake.deprecateInput.Actor)
	}
	if fake.deprecateInput.Reason != "Use v1.1.0 instead." {
		t.Fatalf("reason = %q", fake.deprecateInput.Reason)
	}
	var body moduleVersionDTO
	decodeResponse(t, res, &body)
	if body.DeprecatedAt == nil || body.DeprecatedBy != "test" || body.DeprecationReason != "Use v1.1.0 instead." {
		t.Fatalf("deprecation response = %#v", body)
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
