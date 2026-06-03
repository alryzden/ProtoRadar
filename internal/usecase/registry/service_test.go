package registry

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"strconv"
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

	version, artifact, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader([]byte("proto artifact")),
	})
	if err != nil {
		t.Fatalf("publish version: %v", err)
	}

	if version.ModuleID != module.ID {
		t.Fatalf("module id = %q", version.ModuleID)
	}
	if version.Version.String() != "v1.0.0" {
		t.Fatalf("version = %q", version.Version)
	}
	if artifact.ModuleVersionID != version.ID {
		t.Fatalf("artifact module version id = %q", artifact.ModuleVersionID)
	}
	if artifact.StorageKey != "modules/billing-api/versions/v1.0.0/sha256-1c62ce9153aadfb347569bc35c0f0a44ae7ad685a9616d5c7ce29230370b5e6e.tar.gz" {
		t.Fatalf("storage key = %q", artifact.StorageKey)
	}
	if fixture.store.putKey != artifact.StorageKey {
		t.Fatalf("uploaded key = %q", fixture.store.putKey)
	}
	if len(fixture.versions.byModuleVersion) != 1 {
		t.Fatalf("versions stored = %d", len(fixture.versions.byModuleVersion))
	}
	if len(fixture.artifacts.byVersion) != 1 {
		t.Fatalf("artifacts stored = %d", len(fixture.artifacts.byVersion))
	}
}

func TestPublishModuleVersionWritesOutboxInMetadataTransaction(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	_, _, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader([]byte("proto artifact")),
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
	if len(fixture.versions.byModuleVersion) != 1 || len(fixture.artifacts.byVersion) != 1 {
		t.Fatalf("metadata was not stored with outbox")
	}
}

func TestPublishModuleVersionRejectsUnknownModule(t *testing.T) {
	fixture := newFixture()

	_, _, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader([]byte("proto artifact")),
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
		Artifact:   bytes.NewReader([]byte("proto artifact")),
	}
	if _, _, err := fixture.service.PublishModuleVersion(context.Background(), req); err != nil {
		t.Fatalf("publish version: %v", err)
	}
	req.Artifact = bytes.NewReader([]byte("proto artifact"))
	_, _, err := fixture.service.PublishModuleVersion(context.Background(), req)
	if !errors.Is(err, ErrModuleVersionAlreadyExists) {
		t.Fatalf("error = %v, want ErrModuleVersionAlreadyExists", err)
	}
}

func TestPublishModuleVersionRejectsArtifactLargerThanMaxSize(t *testing.T) {
	fixture := newFixture()
	fixture.service.options.MaxArtifactSizeBytes = 4
	fixture.addModule(t, "billing-api")

	_, _, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
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

	_, _, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader([]byte("proto artifact")),
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if fixture.store.deletedKey != fixture.store.putKey {
		t.Fatalf("deleted key = %q, want uploaded key %q", fixture.store.deletedKey, fixture.store.putKey)
	}
	if len(fixture.versions.byModuleVersion) != 0 {
		t.Fatalf("version metadata was not rolled back")
	}
	if len(fixture.artifacts.byVersion) != 0 {
		t.Fatalf("artifact metadata was not rolled back")
	}
}

func TestDownloadArtifactReturnsStreamAndMetadata(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	_, artifact, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader([]byte("proto artifact")),
	})
	if err != nil {
		t.Fatalf("publish version: %v", err)
	}

	object, gotArtifact, err := fixture.service.DownloadArtifact(context.Background(), "billing-api", "v1.0.0")
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer object.Body.Close()

	if gotArtifact.ID != artifact.ID {
		t.Fatalf("artifact id = %q, want %q", gotArtifact.ID, artifact.ID)
	}
	body, err := io.ReadAll(object.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "proto artifact" {
		t.Fatalf("body = %q", string(body))
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
	tokens         *fakeTokens
	transactions   *fakeTransactions
	outbox         *fakeOutbox
	store          *fakeArtifactStore
	clock          *fakeClock
	ids            *fakeIDs
	tokenGenerator *fakeTokenGenerator
}

func newFixture() *fixture {
	modules := newFakeModules()
	versions := newFakeVersions()
	artifacts := newFakeArtifacts()
	tokens := newFakeTokens()
	outboxWriter := &fakeOutbox{}
	store := &fakeArtifactStore{objects: map[string][]byte{}}
	clock := &fakeClock{now: time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)}
	ids := &fakeIDs{}
	tokenGenerator := &fakeTokenGenerator{next: "raw-token"}
	transactions := &fakeTransactions{
		modules:   modules,
		versions:  versions,
		artifacts: artifacts,
		tokens:    tokens,
		outbox:    outboxWriter,
	}

	return &fixture{
		service: NewService(
			modules,
			versions,
			artifacts,
			tokens,
			transactions,
			outboxWriter,
			store,
			clock,
			ids,
			tokenGenerator,
			Options{MaxArtifactSizeBytes: 1024, TokenHashSecret: "hash-secret"},
		),
		modules:        modules,
		versions:       versions,
		artifacts:      artifacts,
		tokens:         tokens,
		transactions:   transactions,
		outbox:         outboxWriter,
		store:          store,
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
	modules   *fakeModules
	versions  *fakeVersions
	artifacts *fakeArtifacts
	tokens    *fakeTokens
	outbox    *fakeOutbox
}

func (tx *fakeTransactions) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	modules := tx.modules.snapshot()
	versions := tx.versions.snapshot()
	artifacts := tx.artifacts.snapshot()
	tokens := tx.tokens.snapshot()
	outboxRecords := slices.Clone(tx.outbox.records)

	if err := fn(ctx); err != nil {
		tx.modules.restore(modules)
		tx.versions.restore(versions)
		tx.artifacts.restore(artifacts)
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
	byID      map[string]domain.Artifact
	byVersion map[string]domain.Artifact
}

func newFakeArtifacts() *fakeArtifacts {
	return &fakeArtifacts{
		byID:      map[string]domain.Artifact{},
		byVersion: map[string]domain.Artifact{},
	}
}

func (repo *fakeArtifacts) Create(ctx context.Context, artifact domain.Artifact) error {
	if _, exists := repo.byVersion[artifact.ModuleVersionID.String()]; exists {
		return domain.ErrDuplicate
	}
	repo.byID[artifact.ID.String()] = artifact
	repo.byVersion[artifact.ModuleVersionID.String()] = artifact
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
	artifact, exists := repo.byVersion[moduleVersionID.String()]
	if !exists {
		return domain.Artifact{}, domain.ErrNotFound
	}
	return artifact, nil
}

func (repo *fakeArtifacts) snapshot() *fakeArtifacts {
	copy := newFakeArtifacts()
	for key, value := range repo.byID {
		copy.byID[key] = value
	}
	for key, value := range repo.byVersion {
		copy.byVersion[key] = value
	}
	return copy
}

func (repo *fakeArtifacts) restore(snapshot *fakeArtifacts) {
	repo.byID = snapshot.byID
	repo.byVersion = snapshot.byVersion
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
	objects    map[string][]byte
	putKey     string
	deletedKey string
}

func (store *fakeArtifactStore) Put(ctx context.Context, key string, body io.Reader, sizeBytes int64) (storage.ArtifactObject, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return storage.ArtifactObject{}, err
	}
	store.putKey = key
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
	delete(store.objects, key)
	return nil
}
