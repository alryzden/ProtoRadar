package registry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/outbox"
	"github.com/alryzden/ProtoRadar/internal/storage"
)

func TestCreateModuleCreatesModuleAndOutboxInTransaction(t *testing.T) {
	fixture := newFixture()

	module, err := fixture.service.CreateModule(context.Background(), CreateModuleRequest{
		Name:          "billing-api",
		Description:   "Billing contracts",
		RepositoryURL: "https://gitlab.example.com/platform/billing-api",
	})
	if err != nil {
		t.Fatalf("create module: %v", err)
	}

	if module.ID.String() != "module-1" {
		t.Fatalf("module id = %q", module.ID)
	}
	if len(fixture.modules.byName) != 1 {
		t.Fatalf("modules stored = %d", len(fixture.modules.byName))
	}
	if len(fixture.outbox.records) != 1 {
		t.Fatalf("outbox records = %d", len(fixture.outbox.records))
	}
	record := fixture.outbox.records[0]
	if record.EventType != "protoradar.module.created" {
		t.Fatalf("event type = %q", record.EventType)
	}
	if record.DedupKey != "module:module-1:created" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}
}

func TestCreateModuleRollbackPreventsModuleAndOutbox(t *testing.T) {
	fixture := newFixture()
	fixture.outbox.createErr = errors.New("outbox failed")

	_, err := fixture.service.CreateModule(context.Background(), CreateModuleRequest{Name: "billing-api"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(fixture.modules.byName) != 0 {
		t.Fatalf("module was not rolled back")
	}
	if len(fixture.outbox.records) != 0 {
		t.Fatalf("outbox record was not rolled back")
	}
}

func TestCreateModuleRejectsDuplicateModule(t *testing.T) {
	fixture := newFixture()

	if _, err := fixture.service.CreateModule(context.Background(), CreateModuleRequest{Name: "billing-api"}); err != nil {
		t.Fatalf("create module: %v", err)
	}
	_, err := fixture.service.CreateModule(context.Background(), CreateModuleRequest{Name: "billing-api"})
	if !errors.Is(err, ErrModuleAlreadyExists) {
		t.Fatalf("error = %v, want ErrModuleAlreadyExists", err)
	}
}

func TestPublishModuleVersionUploadsAndStoresMetadata(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")

	response, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if err != nil {
		t.Fatalf("publish version: %v", err)
	}

	if response.Version.ModuleID != module.ID {
		t.Fatalf("module id = %q", response.Version.ModuleID)
	}
	if response.Version.Version.String() != "v1.0.0" {
		t.Fatalf("version = %q", response.Version.Version)
	}
	if response.SourceArtifact.ModuleVersionID != response.Version.ID {
		t.Fatalf("source artifact module version id = %q", response.SourceArtifact.ModuleVersionID)
	}
	if response.SourceArtifact.Kind != domain.ArtifactKindSourceArchive {
		t.Fatalf("source artifact kind = %q", response.SourceArtifact.Kind)
	}
	if response.BufImageArtifact.Kind != domain.ArtifactKindBufImage {
		t.Fatalf("buf image artifact kind = %q", response.BufImageArtifact.Kind)
	}
	if response.SourceArtifact.StorageKey == "" || response.BufImageArtifact.StorageKey == "" {
		t.Fatalf("artifact keys should be set: %#v %#v", response.SourceArtifact, response.BufImageArtifact)
	}
	if len(fixture.versions.byModuleVersion) != 1 {
		t.Fatalf("versions stored = %d", len(fixture.versions.byModuleVersion))
	}
	if len(fixture.artifacts.byVersionKind) != 2 {
		t.Fatalf("artifacts stored = %d", len(fixture.artifacts.byVersionKind))
	}
}

func TestPublishModuleVersionWritesOutboxInMetadataTransaction(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if err != nil {
		t.Fatalf("publish version: %v", err)
	}

	if len(fixture.outbox.records) != 1 {
		t.Fatalf("outbox records = %d", len(fixture.outbox.records))
	}
	record := fixture.outbox.records[0]
	if record.EventType != "protoradar.module_version.published" {
		t.Fatalf("event type = %q", record.EventType)
	}
	if record.DedupKey != "module:module-1:version:v1.0.0:published" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}
	if len(fixture.versions.byModuleVersion) != 1 || len(fixture.artifacts.byVersionKind) != 2 {
		t.Fatalf("metadata was not stored with outbox")
	}
}

func TestPublishModuleVersionRejectsUnknownModule(t *testing.T) {
	fixture := newFixture()

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if !errors.Is(err, ErrModuleNotFound) {
		t.Fatalf("error = %v, want ErrModuleNotFound", err)
	}
}

func TestPublishModuleVersionRejectsDuplicateVersion(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	req := PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	}
	if _, err := fixture.service.PublishModuleVersion(context.Background(), req); err != nil {
		t.Fatalf("publish version: %v", err)
	}
	req.Artifact = bytes.NewReader(validSourceArchive(t))
	_, err := fixture.service.PublishModuleVersion(context.Background(), req)
	if !errors.Is(err, ErrModuleVersionAlreadyExists) {
		t.Fatalf("error = %v, want ErrModuleVersionAlreadyExists", err)
	}
}

func TestPublishModuleVersionRejectsArtifactLargerThanMaxSize(t *testing.T) {
	fixture := newFixture()
	fixture.service.options.MaxArtifactSizeBytes = 4
	fixture.addModule(t, "billing-api")

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader([]byte("too large")),
	})
	if !errors.Is(err, ErrArtifactTooLarge) {
		t.Fatalf("error = %v, want ErrArtifactTooLarge", err)
	}
	if fixture.store.putKey != "" {
		t.Fatalf("artifact should not have been uploaded")
	}
}

func TestPublishModuleVersionCleansUpStorageWhenTransactionFailsAfterUpload(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	fixture.outbox.createErr = errors.New("outbox failed")

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(fixture.store.putKeys) != 2 {
		t.Fatalf("put keys = %#v, want 2 uploads", fixture.store.putKeys)
	}
	if len(fixture.store.deletedKeys) != 2 {
		t.Fatalf("deleted keys = %#v, want cleanup of both uploads", fixture.store.deletedKeys)
	}
	if len(fixture.versions.byModuleVersion) != 0 {
		t.Fatalf("version metadata was not rolled back")
	}
	if len(fixture.artifacts.byVersionKind) != 0 {
		t.Fatalf("artifact metadata was not rolled back")
	}
	if len(fixture.bufConfigs.byVersion) != 0 {
		t.Fatalf("buf config metadata was not rolled back")
	}
	if len(fixture.metadata.byVersion) != 0 {
		t.Fatalf("descriptor metadata was not rolled back")
	}
}

func TestPublishModuleVersionRejectsMissingBufYAMLWhenRequired(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(sourceArchiveWithoutBufYAML(t)),
	})
	if !errors.Is(err, ErrBufConfigNotFound) {
		t.Fatalf("error = %v, want ErrBufConfigNotFound", err)
	}
	if len(fixture.versions.byModuleVersion) != 0 || len(fixture.outbox.records) != 0 {
		t.Fatalf("publish should not persist version or outbox")
	}
}

func TestPublishModuleVersionRejectsBufBuildFailure(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	fixture.bufWorkflow.err = errors.New("build failed")
	fixture.bufWorkflow.result = BufWorkflowResult{}

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if !errors.Is(err, ErrBufBuildFailed) {
		t.Fatalf("error = %v, want ErrBufBuildFailed", err)
	}
	if len(fixture.versions.byModuleVersion) != 0 || len(fixture.outbox.records) != 0 {
		t.Fatalf("publish should not persist version or outbox")
	}
}

func TestPublishModuleVersionAllowsLintWarning(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	fixture.bufWorkflow.result.LintResult = domain.BufLintResult{Status: domain.BufLintStatusWarning, Report: "lint warning"}

	response, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if response.LintResult.Status != domain.BufLintStatusWarning || response.LintResult.Report != "lint warning" {
		t.Fatalf("lint result = %#v", response.LintResult)
	}
	if len(fixture.versions.byModuleVersion) != 1 || len(fixture.outbox.records) != 1 {
		t.Fatalf("publish should persist with lint warning")
	}
}

func TestPublishModuleVersionRejectsLintFailureInEnforceMode(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	fixture.service.options.BufLintMode = BufLintModeEnforce
	fixture.bufWorkflow.result.LintResult = domain.BufLintResult{Status: domain.BufLintStatusFailed, Report: "lint failed"}
	fixture.bufWorkflow.err = errors.New("lint failed")

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if !errors.Is(err, ErrBufLintFailed) {
		t.Fatalf("error = %v, want ErrBufLintFailed", err)
	}
	if len(fixture.versions.byModuleVersion) != 0 || len(fixture.outbox.records) != 0 {
		t.Fatalf("publish should not persist version or outbox")
	}
}

func TestPublishModuleVersionStoresDescriptorMetadataAndBufConfig(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	response, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, exists := fixture.bufConfigs.byVersion[response.Version.ID.String()]; !exists {
		t.Fatalf("buf config metadata was not saved")
	}
	metadata, exists := fixture.metadata.byVersion[response.Version.ID.String()]
	if !exists {
		t.Fatalf("descriptor metadata was not saved")
	}
	if response.MetadataSummary != metadata.Summary() {
		t.Fatalf("metadata summary = %#v, want %#v", response.MetadataSummary, metadata.Summary())
	}
}

func TestPublishModuleVersionUploadsSourceArchiveAndBufImage(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	response, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(fixture.store.putKeys) != 2 {
		t.Fatalf("put keys = %#v, want source and buf image", fixture.store.putKeys)
	}
	if !strings.Contains(response.SourceArtifact.StorageKey, "/source/sha256-") || !strings.HasSuffix(response.SourceArtifact.StorageKey, ".tar.gz") {
		t.Fatalf("source key = %q", response.SourceArtifact.StorageKey)
	}
	if !strings.Contains(response.BufImageArtifact.StorageKey, "/buf-image/sha256-") || !strings.HasSuffix(response.BufImageArtifact.StorageKey, ".binpb") {
		t.Fatalf("buf image key = %q", response.BufImageArtifact.StorageKey)
	}
}

func TestPublishModuleVersionRejectsUnsafeArchive(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(buildSourceArchive(t, map[string]string{"../evil.proto": "evil"})),
	})
	if !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("error = %v, want ErrUnsafeArchive", err)
	}
}

func TestPublishModuleVersionEventDoesNotContainRawToken(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	if _, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(fixture.outbox.records) != 1 {
		t.Fatalf("outbox records = %d", len(fixture.outbox.records))
	}
	payload := string(fixture.outbox.records[0].Payload)
	if strings.Contains(payload, "raw-token") || strings.Contains(strings.ToLower(payload), "token") {
		t.Fatalf("event payload contains token data: %s", payload)
	}
	var event map[string]any
	if err := json.Unmarshal(fixture.outbox.records[0].Payload, &event); err != nil {
		t.Fatalf("event payload json: %v", err)
	}
	if event["lint_status"] != string(domain.BufLintStatusPassed) {
		t.Fatalf("lint_status = %#v", event["lint_status"])
	}
}

func TestDownloadArtifactReturnsStreamAndMetadata(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	archive := validSourceArchive(t)
	response, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(archive),
	})
	if err != nil {
		t.Fatalf("publish version: %v", err)
	}

	object, gotArtifact, err := fixture.service.DownloadArtifact(context.Background(), "billing-api", "v1.0.0")
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer object.Body.Close()

	if gotArtifact.ID != response.SourceArtifact.ID {
		t.Fatalf("artifact id = %q, want %q", gotArtifact.ID, response.SourceArtifact.ID)
	}
	body, err := io.ReadAll(object.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.Equal(body, archive) {
		t.Fatalf("downloaded source archive did not match uploaded archive")
	}
}

func TestCreateAPITokenReturnsRawTokenOnceAndStoresOnlyHash(t *testing.T) {
	fixture := newFixture()
	fixture.tokenGenerator.next = "raw-token"

	response, err := fixture.service.CreateAPIToken(context.Background(), CreateAPITokenRequest{Name: "ci"})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	if response.RawToken != "raw-token" {
		t.Fatalf("raw token = %q", response.RawToken)
	}
	if response.Token.TokenHash == "raw-token" {
		t.Fatalf("stored raw token instead of hash")
	}
	stored := fixture.tokens.byHash[response.Token.TokenHash]
	if stored.TokenHash == "" {
		t.Fatalf("token hash was not stored")
	}
	if slices.Contains([]string{stored.TokenHash, response.Token.TokenHash}, "raw-token") {
		t.Fatalf("raw token leaked into stored token")
	}
}

func TestAuthenticateTokenAcceptsValidAndRejectsInvalidOrExpiredToken(t *testing.T) {
	fixture := newFixture()
	fixture.tokenGenerator.next = "raw-token"
	response, err := fixture.service.CreateAPIToken(context.Background(), CreateAPITokenRequest{Name: "ci"})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	subject, err := fixture.service.AuthenticateToken(context.Background(), "raw-token")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if subject.TokenID != response.Token.ID {
		t.Fatalf("subject token id = %q", subject.TokenID)
	}
	if fixture.tokens.lastUsed[response.Token.ID.String()].IsZero() {
		t.Fatalf("last_used_at was not updated")
	}

	if _, err := fixture.service.AuthenticateToken(context.Background(), "wrong-token"); !errors.Is(err, ErrInvalidOrExpiredToken) {
		t.Fatalf("invalid token error = %v", err)
	}

	expiredAt := fixture.clock.now.Add(-time.Hour)
	fixture.tokenGenerator.next = "expired-token"
	if _, err := fixture.service.CreateAPIToken(context.Background(), CreateAPITokenRequest{Name: "expired", ExpiresAt: &expiredAt}); err != nil {
		t.Fatalf("create expired token: %v", err)
	}
	if _, err := fixture.service.AuthenticateToken(context.Background(), "expired-token"); !errors.Is(err, ErrInvalidOrExpiredToken) {
		t.Fatalf("expired token error = %v", err)
	}
}

type fixture struct {
	service        *Service
	modules        *fakeModules
	versions       *fakeVersions
	artifacts      *fakeArtifacts
	bufConfigs     *fakeBufConfigs
	metadata       *fakeMetadata
	tokens         *fakeTokens
	transactions   *fakeTransactions
	outbox         *fakeOutbox
	store          *fakeArtifactStore
	bufWorkflow    *fakeBufWorkflow
	clock          *fakeClock
	ids            *fakeIDs
	tokenGenerator *fakeTokenGenerator
}

func newFixture() *fixture {
	modules := newFakeModules()
	versions := newFakeVersions()
	artifacts := newFakeArtifacts()
	bufConfigs := newFakeBufConfigs()
	metadata := newFakeMetadata()
	tokens := newFakeTokens()
	outboxWriter := &fakeOutbox{}
	store := &fakeArtifactStore{objects: map[string][]byte{}}
	bufWorkflow := &fakeBufWorkflow{result: successfulBufWorkflowResult()}
	clock := &fakeClock{now: time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)}
	ids := &fakeIDs{}
	tokenGenerator := &fakeTokenGenerator{next: "raw-token"}
	transactions := &fakeTransactions{
		modules:    modules,
		versions:   versions,
		artifacts:  artifacts,
		bufConfigs: bufConfigs,
		metadata:   metadata,
		tokens:     tokens,
		outbox:     outboxWriter,
	}

	return &fixture{
		service: NewService(
			modules,
			versions,
			artifacts,
			bufConfigs,
			metadata,
			tokens,
			transactions,
			outboxWriter,
			store,
			bufWorkflow,
			clock,
			ids,
			tokenGenerator,
			Options{MaxArtifactSizeBytes: 4096, MaxSourceUncompressedSizeBytes: 4096, TokenHashSecret: "hash-secret", BufRequireConfig: true, BufLintMode: BufLintModeWarn},
		),
		modules:        modules,
		versions:       versions,
		artifacts:      artifacts,
		bufConfigs:     bufConfigs,
		metadata:       metadata,
		tokens:         tokens,
		transactions:   transactions,
		outbox:         outboxWriter,
		store:          store,
		bufWorkflow:    bufWorkflow,
		clock:          clock,
		ids:            ids,
		tokenGenerator: tokenGenerator,
	}
}

func (fixture *fixture) addModule(t *testing.T, nameValue string) domain.Module {
	t.Helper()

	name, err := domain.NewModuleName(nameValue)
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	module := domain.Module{
		ID:        fixture.nextModuleID(t),
		Name:      name,
		CreatedAt: fixture.clock.now,
		UpdatedAt: fixture.clock.now,
	}
	if err := fixture.modules.Create(context.Background(), module); err != nil {
		t.Fatalf("create fake module: %v", err)
	}
	return module
}

type fakeClock struct {
	now time.Time
}

func (clock *fakeClock) Now() time.Time {
	return clock.now
}

type fakeIDs struct {
	moduleID        int
	moduleVersionID int
	artifactID      int
	apiTokenID      int
}

func (fixture *fixture) nextModuleID(t *testing.T) domain.ModuleID {
	t.Helper()
	id, err := fixture.ids.NewModuleID()
	if err != nil {
		t.Fatalf("module id: %v", err)
	}
	return id
}

func (ids *fakeIDs) NewModuleID() (domain.ModuleID, error) {
	ids.moduleID++
	return domain.NewModuleID("module-" + strconv.Itoa(ids.moduleID)), nil
}

func (ids *fakeIDs) NewModuleVersionID() (domain.ModuleVersionID, error) {
	ids.moduleVersionID++
	return domain.NewModuleVersionID("module-version-" + strconv.Itoa(ids.moduleVersionID)), nil
}

func (ids *fakeIDs) NewArtifactID() (domain.ArtifactID, error) {
	ids.artifactID++
	return domain.NewArtifactID("artifact-" + strconv.Itoa(ids.artifactID)), nil
}

func (ids *fakeIDs) NewAPITokenID() (domain.APITokenID, error) {
	ids.apiTokenID++
	return domain.NewAPITokenID("api-token-" + strconv.Itoa(ids.apiTokenID)), nil
}

type fakeTokenGenerator struct {
	next string
}

func (generator *fakeTokenGenerator) NewToken() (string, error) {
	return generator.next, nil
}

type fakeTransactions struct {
	modules    *fakeModules
	versions   *fakeVersions
	artifacts  *fakeArtifacts
	bufConfigs *fakeBufConfigs
	metadata   *fakeMetadata
	tokens     *fakeTokens
	outbox     *fakeOutbox
}

func (tx *fakeTransactions) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	modules := tx.modules.snapshot()
	versions := tx.versions.snapshot()
	artifacts := tx.artifacts.snapshot()
	bufConfigs := tx.bufConfigs.snapshot()
	metadata := tx.metadata.snapshot()
	tokens := tx.tokens.snapshot()
	outboxRecords := slices.Clone(tx.outbox.records)

	if err := fn(ctx); err != nil {
		tx.modules.restore(modules)
		tx.versions.restore(versions)
		tx.artifacts.restore(artifacts)
		tx.bufConfigs.restore(bufConfigs)
		tx.metadata.restore(metadata)
		tx.tokens.restore(tokens)
		tx.outbox.records = outboxRecords
		return err
	}
	return nil
}

type fakeModules struct {
	byID   map[string]domain.Module
	byName map[string]domain.Module
}

func newFakeModules() *fakeModules {
	return &fakeModules{
		byID:   map[string]domain.Module{},
		byName: map[string]domain.Module{},
	}
}

func (repo *fakeModules) Create(ctx context.Context, module domain.Module) error {
	if _, exists := repo.byName[module.Name.String()]; exists {
		return domain.ErrDuplicate
	}
	repo.byID[module.ID.String()] = module
	repo.byName[module.Name.String()] = module
	return nil
}

func (repo *fakeModules) GetByID(ctx context.Context, id domain.ModuleID) (domain.Module, error) {
	module, exists := repo.byID[id.String()]
	if !exists {
		return domain.Module{}, domain.ErrNotFound
	}
	return module, nil
}

func (repo *fakeModules) GetByName(ctx context.Context, name domain.ModuleName) (domain.Module, error) {
	module, exists := repo.byName[name.String()]
	if !exists {
		return domain.Module{}, domain.ErrNotFound
	}
	return module, nil
}

func (repo *fakeModules) List(ctx context.Context, limit int, offset int) ([]domain.Module, error) {
	modules := make([]domain.Module, 0, len(repo.byName))
	for _, module := range repo.byName {
		modules = append(modules, module)
	}
	return modules, nil
}

func (repo *fakeModules) snapshot() *fakeModules {
	copy := newFakeModules()
	for key, value := range repo.byID {
		copy.byID[key] = value
	}
	for key, value := range repo.byName {
		copy.byName[key] = value
	}
	return copy
}

func (repo *fakeModules) restore(snapshot *fakeModules) {
	repo.byID = snapshot.byID
	repo.byName = snapshot.byName
}

type fakeVersions struct {
	byID            map[string]domain.ModuleVersion
	byModuleVersion map[string]domain.ModuleVersion
}

func newFakeVersions() *fakeVersions {
	return &fakeVersions{
		byID:            map[string]domain.ModuleVersion{},
		byModuleVersion: map[string]domain.ModuleVersion{},
	}
}

func (repo *fakeVersions) Create(ctx context.Context, version domain.ModuleVersion) error {
	key := moduleVersionKey(version.ModuleID, version.Version)
	if _, exists := repo.byModuleVersion[key]; exists {
		return domain.ErrDuplicate
	}
	repo.byID[version.ID.String()] = version
	repo.byModuleVersion[key] = version
	return nil
}

func (repo *fakeVersions) GetByID(ctx context.Context, id domain.ModuleVersionID) (domain.ModuleVersion, error) {
	version, exists := repo.byID[id.String()]
	if !exists {
		return domain.ModuleVersion{}, domain.ErrNotFound
	}
	return version, nil
}

func (repo *fakeVersions) GetByModuleAndVersion(ctx context.Context, moduleID domain.ModuleID, version domain.Version) (domain.ModuleVersion, error) {
	moduleVersion, exists := repo.byModuleVersion[moduleVersionKey(moduleID, version)]
	if !exists {
		return domain.ModuleVersion{}, domain.ErrNotFound
	}
	return moduleVersion, nil
}

func (repo *fakeVersions) GetLatestByModule(ctx context.Context, moduleID domain.ModuleID) (domain.ModuleVersion, error) {
	var latest domain.ModuleVersion
	for _, version := range repo.byModuleVersion {
		if version.ModuleID == moduleID && (latest.ID == "" || version.CreatedAt.After(latest.CreatedAt)) {
			latest = version
		}
	}
	if latest.ID == "" {
		return domain.ModuleVersion{}, domain.ErrNotFound
	}
	return latest, nil
}

func (repo *fakeVersions) ListByModule(ctx context.Context, moduleID domain.ModuleID, limit int, offset int) ([]domain.ModuleVersion, error) {
	versions := make([]domain.ModuleVersion, 0)
	for _, version := range repo.byModuleVersion {
		if version.ModuleID == moduleID {
			versions = append(versions, version)
		}
	}
	return versions, nil
}

func (repo *fakeVersions) snapshot() *fakeVersions {
	copy := newFakeVersions()
	for key, value := range repo.byID {
		copy.byID[key] = value
	}
	for key, value := range repo.byModuleVersion {
		copy.byModuleVersion[key] = value
	}
	return copy
}

func (repo *fakeVersions) restore(snapshot *fakeVersions) {
	repo.byID = snapshot.byID
	repo.byModuleVersion = snapshot.byModuleVersion
}

func moduleVersionKey(moduleID domain.ModuleID, version domain.Version) string {
	return moduleID.String() + ":" + version.String()
}

type fakeArtifacts struct {
	byID          map[string]domain.Artifact
	byVersionKind map[string]domain.Artifact
}

func newFakeArtifacts() *fakeArtifacts {
	return &fakeArtifacts{
		byID:          map[string]domain.Artifact{},
		byVersionKind: map[string]domain.Artifact{},
	}
}

func (repo *fakeArtifacts) Create(ctx context.Context, artifact domain.Artifact) error {
	key := artifactVersionKindKey(artifact.ModuleVersionID, artifact.Kind)
	if _, exists := repo.byVersionKind[key]; exists {
		return domain.ErrDuplicate
	}
	repo.byID[artifact.ID.String()] = artifact
	repo.byVersionKind[key] = artifact
	return nil
}

func (repo *fakeArtifacts) GetByID(ctx context.Context, id domain.ArtifactID) (domain.Artifact, error) {
	artifact, exists := repo.byID[id.String()]
	if !exists {
		return domain.Artifact{}, domain.ErrNotFound
	}
	return artifact, nil
}

func (repo *fakeArtifacts) GetByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) (domain.Artifact, error) {
	return repo.GetByModuleVersionAndKind(ctx, moduleVersionID, domain.ArtifactKindSourceArchive)
}

func (repo *fakeArtifacts) GetByModuleVersionAndKind(ctx context.Context, moduleVersionID domain.ModuleVersionID, kind domain.ArtifactKind) (domain.Artifact, error) {
	artifact, exists := repo.byVersionKind[artifactVersionKindKey(moduleVersionID, kind)]
	if !exists {
		return domain.Artifact{}, domain.ErrNotFound
	}
	return artifact, nil
}

func (repo *fakeArtifacts) ListByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) ([]domain.Artifact, error) {
	artifacts := make([]domain.Artifact, 0)
	for _, artifact := range repo.byVersionKind {
		if artifact.ModuleVersionID == moduleVersionID {
			artifacts = append(artifacts, artifact)
		}
	}
	return artifacts, nil
}

func (repo *fakeArtifacts) snapshot() *fakeArtifacts {
	copy := newFakeArtifacts()
	for key, value := range repo.byID {
		copy.byID[key] = value
	}
	for key, value := range repo.byVersionKind {
		copy.byVersionKind[key] = value
	}
	return copy
}

func (repo *fakeArtifacts) restore(snapshot *fakeArtifacts) {
	repo.byID = snapshot.byID
	repo.byVersionKind = snapshot.byVersionKind
}

func artifactVersionKindKey(moduleVersionID domain.ModuleVersionID, kind domain.ArtifactKind) string {
	return moduleVersionID.String() + ":" + kind.String()
}

type fakeBufConfigs struct {
	byVersion map[string]domain.BufConfigInfo
}

func newFakeBufConfigs() *fakeBufConfigs {
	return &fakeBufConfigs{byVersion: map[string]domain.BufConfigInfo{}}
}

func (repo *fakeBufConfigs) Save(ctx context.Context, moduleVersionID domain.ModuleVersionID, config domain.BufConfigInfo) error {
	repo.byVersion[moduleVersionID.String()] = config
	return nil
}

func (repo *fakeBufConfigs) GetByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) (domain.BufConfigInfo, error) {
	config, exists := repo.byVersion[moduleVersionID.String()]
	if !exists {
		return domain.BufConfigInfo{}, domain.ErrNotFound
	}
	return config, nil
}

func (repo *fakeBufConfigs) snapshot() *fakeBufConfigs {
	copy := newFakeBufConfigs()
	for key, value := range repo.byVersion {
		copy.byVersion[key] = value
	}
	return copy
}

func (repo *fakeBufConfigs) restore(snapshot *fakeBufConfigs) {
	repo.byVersion = snapshot.byVersion
}

type fakeMetadata struct {
	byVersion map[string]domain.DescriptorMetadata
	saveErr   error
}

func newFakeMetadata() *fakeMetadata {
	return &fakeMetadata{byVersion: map[string]domain.DescriptorMetadata{}}
}

func (repo *fakeMetadata) Save(ctx context.Context, moduleVersionID domain.ModuleVersionID, metadata domain.DescriptorMetadata) error {
	if repo.saveErr != nil {
		return repo.saveErr
	}
	repo.byVersion[moduleVersionID.String()] = metadata
	return nil
}

func (repo *fakeMetadata) GetByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) (domain.DescriptorMetadata, error) {
	metadata, exists := repo.byVersion[moduleVersionID.String()]
	if !exists {
		return domain.DescriptorMetadata{}, domain.ErrNotFound
	}
	return metadata, nil
}

func (repo *fakeMetadata) GetSummaryByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) (domain.DescriptorMetadataSummary, error) {
	metadata, err := repo.GetByModuleVersion(ctx, moduleVersionID)
	if err != nil {
		return domain.DescriptorMetadataSummary{}, err
	}
	return metadata.Summary(), nil
}

func (repo *fakeMetadata) snapshot() *fakeMetadata {
	copy := newFakeMetadata()
	copy.saveErr = repo.saveErr
	for key, value := range repo.byVersion {
		copy.byVersion[key] = value
	}
	return copy
}

func (repo *fakeMetadata) restore(snapshot *fakeMetadata) {
	repo.byVersion = snapshot.byVersion
	repo.saveErr = snapshot.saveErr
}

type fakeTokens struct {
	byID     map[string]domain.APIToken
	byHash   map[string]domain.APIToken
	lastUsed map[string]time.Time
}

func newFakeTokens() *fakeTokens {
	return &fakeTokens{
		byID:     map[string]domain.APIToken{},
		byHash:   map[string]domain.APIToken{},
		lastUsed: map[string]time.Time{},
	}
}

func (repo *fakeTokens) Create(ctx context.Context, token domain.APIToken) error {
	if _, exists := repo.byHash[token.TokenHash]; exists {
		return domain.ErrDuplicate
	}
	repo.byID[token.ID.String()] = token
	repo.byHash[token.TokenHash] = token
	return nil
}

func (repo *fakeTokens) GetByID(ctx context.Context, id domain.APITokenID) (domain.APIToken, error) {
	token, exists := repo.byID[id.String()]
	if !exists {
		return domain.APIToken{}, domain.ErrNotFound
	}
	return token, nil
}

func (repo *fakeTokens) GetByHash(ctx context.Context, tokenHash string) (domain.APIToken, error) {
	token, exists := repo.byHash[tokenHash]
	if !exists {
		return domain.APIToken{}, domain.ErrNotFound
	}
	return token, nil
}

func (repo *fakeTokens) MarkUsed(ctx context.Context, id domain.APITokenID, usedAt time.Time) error {
	token, exists := repo.byID[id.String()]
	if !exists {
		return domain.ErrNotFound
	}
	token.LastUsedAt = &usedAt
	repo.byID[id.String()] = token
	repo.byHash[token.TokenHash] = token
	repo.lastUsed[id.String()] = usedAt
	return nil
}

func (repo *fakeTokens) snapshot() *fakeTokens {
	copy := newFakeTokens()
	for key, value := range repo.byID {
		copy.byID[key] = value
	}
	for key, value := range repo.byHash {
		copy.byHash[key] = value
	}
	for key, value := range repo.lastUsed {
		copy.lastUsed[key] = value
	}
	return copy
}

func (repo *fakeTokens) restore(snapshot *fakeTokens) {
	repo.byID = snapshot.byID
	repo.byHash = snapshot.byHash
	repo.lastUsed = snapshot.lastUsed
}

type fakeOutbox struct {
	records   []outbox.Record
	createErr error
}

func (writer *fakeOutbox) Create(ctx context.Context, record outbox.Record) error {
	if writer.createErr != nil {
		return writer.createErr
	}
	writer.records = append(writer.records, record)
	return nil
}

type fakeArtifactStore struct {
	objects     map[string][]byte
	putKey      string
	deletedKey  string
	putKeys     []string
	deletedKeys []string
}

func (store *fakeArtifactStore) Put(ctx context.Context, key string, body io.Reader, sizeBytes int64) (storage.ArtifactObject, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return storage.ArtifactObject{}, err
	}
	store.putKey = key
	store.putKeys = append(store.putKeys, key)
	store.objects[key] = data
	return storage.ArtifactObject{
		Key:         key,
		ContentType: "application/gzip",
		SizeBytes:   sizeBytes,
	}, nil
}

func (store *fakeArtifactStore) Get(ctx context.Context, key string) (storage.ArtifactObject, error) {
	data, exists := store.objects[key]
	if !exists {
		return storage.ArtifactObject{}, domain.ErrNotFound
	}
	return storage.ArtifactObject{
		Key:         key,
		ContentType: "application/gzip",
		SizeBytes:   int64(len(data)),
		Body:        io.NopCloser(bytes.NewReader(data)),
	}, nil
}

func (store *fakeArtifactStore) Delete(ctx context.Context, key string) error {
	store.deletedKey = key
	store.deletedKeys = append(store.deletedKeys, key)
	delete(store.objects, key)
	return nil
}

type fakeBufWorkflow struct {
	result registryBufWorkflowResultAlias
	err    error
	calls  []bufWorkflowCall
}

type registryBufWorkflowResultAlias = BufWorkflowResult

type bufWorkflowCall struct {
	workdir string
	options BufWorkflowOptions
}

func (workflow *fakeBufWorkflow) Inspect(ctx context.Context, workdir string, options BufWorkflowOptions) (BufWorkflowResult, error) {
	workflow.calls = append(workflow.calls, bufWorkflowCall{workdir: workdir, options: options})
	return workflow.result, workflow.err
}

func successfulBufWorkflowResult() BufWorkflowResult {
	metadata := domain.DescriptorMetadata{Files: []domain.ProtoFile{
		{
			Path:        "user.proto",
			PackageName: "user.v1",
			Syntax:      "proto3",
			Messages: []domain.ProtoMessage{
				{
					Name:     "User",
					FullName: "user.v1.User",
					Fields: []domain.ProtoField{
						{Name: "id", Number: 1, Type: "TYPE_STRING", Label: "LABEL_OPTIONAL", JSONName: "id"},
					},
				},
			},
		},
	}}
	return BufWorkflowResult{
		ConfigInfo: domain.BufConfigInfo{
			BufYAMLPresent: true,
			BufLockPresent: true,
			BufYAMLDigest:  "sha256:buf-yaml",
			BufLockDigest:  "sha256:buf-lock",
			LintEnabled:    true,
		},
		BufImage:           []byte("buf image"),
		BufImageDigest:     "sha256:buf-image",
		LintResult:         domain.BufLintResult{Status: domain.BufLintStatusPassed},
		DescriptorMetadata: metadata,
	}
}

func validSourceArchive(t *testing.T) []byte {
	t.Helper()
	return buildSourceArchive(t, map[string]string{
		"buf.yaml":   "version: v2\n",
		"user.proto": "syntax = \"proto3\";",
	})
}

func sourceArchiveWithoutBufYAML(t *testing.T) []byte {
	t.Helper()
	return buildSourceArchive(t, map[string]string{
		"user.proto": "syntax = \"proto3\";",
	})
}

func buildSourceArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, body := range files {
		if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(body))}); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tarWriter.Write([]byte(body)); err != nil {
			t.Fatalf("write tar body: %v", err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buffer.Bytes()
}
