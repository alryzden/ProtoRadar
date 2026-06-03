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

func newServerWithPublishError(err error) http.Handler {
	fake := newFakeRegistry()
	fake.publishErr = err
	_, _ = fake.CreateModule(context.Background(), registry.CreateModuleRequest{Name: "user-api"})
	return NewServer(fake, Options{BootstrapToken: "bootstrap"}).Handler()
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
	now         time.Time
	modules     map[string]domain.Module
	versions    map[string]domain.ModuleVersion
	artifacts   map[string][]domain.Artifact
	objects     map[string][]byte
	bufConfigs  map[string]domain.BufConfigInfo
	lintResults map[string]domain.BufLintResult
	metadata    map[string]domain.DescriptorMetadata
	publishErr  error
	publishLint domain.BufLintResult
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{
		now:         time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC),
		modules:     map[string]domain.Module{},
		versions:    map[string]domain.ModuleVersion{},
		artifacts:   map[string][]domain.Artifact{},
		objects:     map[string][]byte{},
		bufConfigs:  map[string]domain.BufConfigInfo{},
		lintResults: map[string]domain.BufLintResult{},
		metadata:    map[string]domain.DescriptorMetadata{},
		publishLint: domain.BufLintResult{Status: domain.BufLintStatusPassed},
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
