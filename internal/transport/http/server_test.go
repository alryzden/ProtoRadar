package httptransport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/storage"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
)

func TestUnauthorizedWithoutToken(t *testing.T) {
	server := newTestServer()

	res := request(t, server, http.MethodGet, "/api/v1/modules", nil, "", "")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
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
	ready := request(t, server, http.MethodGet, "/readyz", nil, "", "")
	if ready.Code != http.StatusOK {
		t.Fatalf("ready status = %d", ready.Code)
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

func TestPublishModuleVersion(t *testing.T) {
	server := newTestServer()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")

	res := multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}

	var body publishModuleVersionResponse
	decodeResponse(t, res, &body)
	if body.Version.Version != "v1.0.0" {
		t.Fatalf("version = %q", body.Version.Version)
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
	return NewServer(newFakeRegistry(), Options{BootstrapToken: "bootstrap"}).Handler()
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

func decodeResponse(t *testing.T, res *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.NewDecoder(res.Body).Decode(dst); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

type fakeRegistry struct {
	now       time.Time
	modules   map[string]domain.Module
	versions  map[string]domain.ModuleVersion
	artifacts map[string]domain.Artifact
	objects   map[string][]byte
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{
		now:       time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC),
		modules:   map[string]domain.Module{},
		versions:  map[string]domain.ModuleVersion{},
		artifacts: map[string]domain.Artifact{},
		objects:   map[string][]byte{},
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

func (fake *fakeRegistry) PublishModuleVersion(ctx context.Context, req registry.PublishModuleVersionRequest) (domain.ModuleVersion, domain.Artifact, error) {
	module, exists := fake.modules[req.ModuleName]
	if !exists {
		return domain.ModuleVersion{}, domain.Artifact{}, registry.ErrModuleNotFound
	}
	versionValue, err := domain.NewVersion(req.Version)
	if err != nil {
		return domain.ModuleVersion{}, domain.Artifact{}, registry.ErrInvalidVersion
	}
	key := req.ModuleName + ":" + req.Version
	if _, exists := fake.versions[key]; exists {
		return domain.ModuleVersion{}, domain.Artifact{}, registry.ErrModuleVersionAlreadyExists
	}
	body, err := io.ReadAll(req.Artifact)
	if err != nil {
		return domain.ModuleVersion{}, domain.Artifact{}, err
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
	artifact := domain.Artifact{
		ID:              domain.NewArtifactID("artifact-" + req.Version),
		ModuleVersionID: moduleVersion.ID,
		StorageKey:      "artifact-key-" + req.Version,
		ChecksumSHA256:  checksum,
		SizeBytes:       int64(len(body)),
		CreatedAt:       fake.now,
	}
	fake.versions[key] = moduleVersion
	fake.artifacts[key] = artifact
	fake.objects[key] = body
	return moduleVersion, artifact, nil
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

func (fake *fakeRegistry) DownloadArtifact(ctx context.Context, moduleName string, version string) (storage.ArtifactObject, domain.Artifact, error) {
	key := moduleName + ":" + version
	artifact, exists := fake.artifacts[key]
	if !exists {
		return storage.ArtifactObject{}, domain.Artifact{}, registry.ErrModuleNotFound
	}
	body := fake.objects[key]
	return storage.ArtifactObject{
		Key:         artifact.StorageKey,
		ContentType: "application/gzip",
		SizeBytes:   int64(len(body)),
		Body:        io.NopCloser(bytes.NewReader(body)),
	}, artifact, nil
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
