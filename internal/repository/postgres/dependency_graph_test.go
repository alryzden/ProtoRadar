package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/outbox"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
)

func TestModuleDependencyRepositoryInsertAndListUpstream(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewModuleDependencyRepository(db)
	consumer, consumerVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010001", "billing-api", "00000000-0000-0000-0000-000000020001", "v1.0.0", time.Now().UTC())
	provider, providerVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010002", "user-api", "00000000-0000-0000-0000-000000020002", "v1.0.0", time.Now().UTC())
	dependency := testModuleDependency(t, "00000000-0000-0000-0000-000000030001", consumer, consumerVersion, provider, providerVersion)

	if err := repo.ReplaceByConsumerModuleVersion(ctx, consumerVersion.ID, []domain.ModuleDependency{dependency}, nil); err != nil {
		t.Fatalf("replace dependencies: %v", err)
	}

	upstream, err := repo.ListUpstreamByModuleVersion(ctx, consumerVersion.ID)
	if err != nil {
		t.Fatalf("list upstream by version: %v", err)
	}
	if len(upstream) != 1 {
		t.Fatalf("upstream = %d, want 1", len(upstream))
	}
	assertModuleDependency(t, upstream[0], dependency)

	byModule, err := repo.ListUpstreamByModule(ctx, consumer.ID)
	if err != nil {
		t.Fatalf("list upstream by module: %v", err)
	}
	if len(byModule) != 1 {
		t.Fatalf("upstream by module = %d, want 1", len(byModule))
	}
	assertModuleDependency(t, byModule[0], dependency)
}

func TestModuleDependencyRepositoryListDownstream(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewModuleDependencyRepository(db)
	consumer, consumerVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010003", "billing-api", "00000000-0000-0000-0000-000000020003", "v1.0.0", time.Now().UTC())
	provider, providerVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010004", "user-api", "00000000-0000-0000-0000-000000020004", "v1.0.0", time.Now().UTC())
	dependency := testModuleDependency(t, "00000000-0000-0000-0000-000000030002", consumer, consumerVersion, provider, providerVersion)

	if err := repo.ReplaceByConsumerModuleVersion(ctx, consumerVersion.ID, []domain.ModuleDependency{dependency}, nil); err != nil {
		t.Fatalf("replace dependencies: %v", err)
	}
	downstream, err := repo.ListDownstreamByModule(ctx, provider.ID)
	if err != nil {
		t.Fatalf("list downstream: %v", err)
	}
	if len(downstream) != 1 {
		t.Fatalf("downstream = %d, want 1", len(downstream))
	}
	assertModuleDependency(t, downstream[0], dependency)
}

func TestModuleDependencyRepositoryReplaceDeletesOldEdges(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewModuleDependencyRepository(db)
	consumer, consumerVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010005", "billing-api", "00000000-0000-0000-0000-000000020005", "v1.0.0", time.Now().UTC())
	firstProvider, firstProviderVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010006", "user-api", "00000000-0000-0000-0000-000000020006", "v1.0.0", time.Now().UTC())
	secondProvider, secondProviderVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010007", "account-api", "00000000-0000-0000-0000-000000020007", "v1.0.0", time.Now().UTC())
	oldDependency := testModuleDependency(t, "00000000-0000-0000-0000-000000030003", consumer, consumerVersion, firstProvider, firstProviderVersion)
	newDependency := testModuleDependency(t, "00000000-0000-0000-0000-000000030004", consumer, consumerVersion, secondProvider, secondProviderVersion)
	newDependency.ImportPath = "account/v1/account.proto"
	newDependency.ReferencedPackage = "account.v1"
	newDependency.ReferencedSymbol = "account.v1.Account"

	if err := repo.ReplaceByConsumerModuleVersion(ctx, consumerVersion.ID, []domain.ModuleDependency{oldDependency}, nil); err != nil {
		t.Fatalf("replace old dependencies: %v", err)
	}
	if err := repo.ReplaceByConsumerModuleVersion(ctx, consumerVersion.ID, []domain.ModuleDependency{newDependency}, nil); err != nil {
		t.Fatalf("replace new dependencies: %v", err)
	}

	upstream, err := repo.ListUpstreamByModuleVersion(ctx, consumerVersion.ID)
	if err != nil {
		t.Fatalf("list upstream: %v", err)
	}
	if len(upstream) != 1 {
		t.Fatalf("upstream = %d, want 1", len(upstream))
	}
	assertModuleDependency(t, upstream[0], newDependency)
}

func TestModuleDependencyRepositoryReplaceInsertsUnresolvedDependencies(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewModuleDependencyRepository(db)
	module, version := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010008", "billing-api", "00000000-0000-0000-0000-000000020008", "v1.0.0", time.Now().UTC())
	unresolved := testUnresolvedDependency(t, "00000000-0000-0000-0000-000000040001", module, version)

	if err := repo.ReplaceByConsumerModuleVersion(ctx, version.ID, nil, []domain.UnresolvedProtoDependency{unresolved}); err != nil {
		t.Fatalf("replace unresolved: %v", err)
	}
	items, err := repo.ListUnresolvedByModuleVersion(ctx, version.ID)
	if err != nil {
		t.Fatalf("list unresolved by version: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("unresolved = %d, want 1", len(items))
	}
	assertUnresolvedDependency(t, items[0], unresolved)

	byModule, err := repo.ListUnresolvedByModule(ctx, module.ID)
	if err != nil {
		t.Fatalf("list unresolved by module: %v", err)
	}
	if len(byModule) != 1 {
		t.Fatalf("unresolved by module = %d, want 1", len(byModule))
	}
	assertUnresolvedDependency(t, byModule[0], unresolved)
}

func TestModuleDependencyRepositoryUniqueEdgeConstraint(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewModuleDependencyRepository(db)
	consumer, consumerVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010009", "billing-api", "00000000-0000-0000-0000-000000020009", "v1.0.0", time.Now().UTC())
	provider, providerVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010010", "user-api", "00000000-0000-0000-0000-000000020010", "v1.0.0", time.Now().UTC())
	first := testModuleDependency(t, "00000000-0000-0000-0000-000000030005", consumer, consumerVersion, provider, providerVersion)
	duplicate := first
	duplicate.ID = domain.NewModuleDependencyID("00000000-0000-0000-0000-000000030006")

	err := repo.ReplaceByConsumerModuleVersion(ctx, consumerVersion.ID, []domain.ModuleDependency{first, duplicate}, nil)
	if !errors.Is(err, domain.ErrDuplicate) {
		t.Fatalf("duplicate edge error = %v, want ErrDuplicate", err)
	}
	upstream, err := repo.ListUpstreamByModuleVersion(ctx, consumerVersion.ID)
	if err != nil {
		t.Fatalf("list upstream: %v", err)
	}
	if len(upstream) != 0 {
		t.Fatalf("duplicate transaction should roll back, got %d edges", len(upstream))
	}
}

func TestModuleDependencyRepositoryRejectsSelfEdge(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewModuleDependencyRepository(db)
	module, version := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010011", "billing-api", "00000000-0000-0000-0000-000000020011", "v1.0.0", time.Now().UTC())
	dependency := testModuleDependency(t, "00000000-0000-0000-0000-000000030007", module, version, module, version)

	if err := repo.ReplaceByConsumerModuleVersion(ctx, version.ID, []domain.ModuleDependency{dependency}, nil); err == nil {
		t.Fatalf("expected self dependency to fail")
	}
	assertTableCount(t, ctx, db, "module_dependencies", 0)
}

func TestModuleDependencyRepositoryListAffectedModulesUsesLatestConsumers(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewModuleDependencyRepository(db)
	now := time.Now().UTC()
	provider, providerVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010012", "user-api", "00000000-0000-0000-0000-000000020012", "v1.0.0", now)
	consumer, oldConsumerVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010013", "billing-api", "00000000-0000-0000-0000-000000020013", "v1.0.0", now.Add(-time.Hour))
	_, latestConsumerVersion := createGraphModuleVersionForModule(t, ctx, db, consumer, "00000000-0000-0000-0000-000000020014", "v1.1.0", now)
	oldDependency := testModuleDependency(t, "00000000-0000-0000-0000-000000030008", consumer, oldConsumerVersion, provider, providerVersion)
	latestDependency := testModuleDependency(t, "00000000-0000-0000-0000-000000030009", consumer, latestConsumerVersion, provider, providerVersion)
	latestDependency.Source = domain.DependencySourceTypeReference
	latestDependency.Reason = domain.DependencyResolutionReasonSymbol
	latestDependency.ImportPath = ""
	latestDependency.ReferencedSymbol = "user.v1.User"

	if err := repo.ReplaceByConsumerModuleVersion(ctx, oldConsumerVersion.ID, []domain.ModuleDependency{oldDependency}, nil); err != nil {
		t.Fatalf("replace old dependencies: %v", err)
	}
	if err := repo.ReplaceByConsumerModuleVersion(ctx, latestConsumerVersion.ID, []domain.ModuleDependency{latestDependency}, nil); err != nil {
		t.Fatalf("replace latest dependencies: %v", err)
	}

	affected, err := repo.ListAffectedModules(ctx, provider.ID)
	if err != nil {
		t.Fatalf("list affected: %v", err)
	}
	if len(affected) != 1 {
		t.Fatalf("affected = %d, want 1", len(affected))
	}
	if affected[0].ModuleName.String() != "billing-api" || affected[0].LatestVersion.String() != "v1.1.0" {
		t.Fatalf("affected module = %#v", affected[0])
	}
	if len(affected[0].DependencySources) != 1 || affected[0].DependencySources[0] != domain.DependencySourceTypeReference {
		t.Fatalf("affected sources = %#v", affected[0].DependencySources)
	}
	if len(affected[0].Reasons) != 1 || affected[0].Reasons[0] != domain.DependencyResolutionReasonSymbol {
		t.Fatalf("affected reasons = %#v", affected[0].Reasons)
	}
}

func TestModuleDependencyRepositoryTransactionRollback(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewModuleDependencyRepository(db)
	consumer, consumerVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010014", "billing-api", "00000000-0000-0000-0000-000000020015", "v1.0.0", time.Now().UTC())
	provider, providerVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010015", "user-api", "00000000-0000-0000-0000-000000020016", "v1.0.0", time.Now().UTC())
	dependency := testModuleDependency(t, "00000000-0000-0000-0000-000000030010", consumer, consumerVersion, provider, providerVersion)
	unresolved := testUnresolvedDependency(t, "00000000-0000-0000-0000-000000040002", consumer, consumerVersion)
	rollbackErr := errors.New("force rollback")

	err := db.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := repo.ReplaceByConsumerModuleVersion(txCtx, consumerVersion.ID, []domain.ModuleDependency{dependency}, []domain.UnresolvedProtoDependency{unresolved}); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("transaction error = %v", err)
	}
	assertTableCount(t, ctx, db, "module_dependencies", 0)
	assertTableCount(t, ctx, db, "unresolved_proto_dependencies", 0)
}

func TestModuleDependencyRepositoryCanShareTransactionWithOutbox(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewModuleDependencyRepository(db)
	outboxWriter := NewOutboxWriter(db)
	consumer, consumerVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010016", "billing-api", "00000000-0000-0000-0000-000000020017", "v1.0.0", time.Now().UTC())
	provider, providerVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010017", "user-api", "00000000-0000-0000-0000-000000020018", "v1.0.0", time.Now().UTC())
	dependency := testModuleDependency(t, "00000000-0000-0000-0000-000000030011", consumer, consumerVersion, provider, providerVersion)

	err := db.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := repo.ReplaceByConsumerModuleVersion(txCtx, consumerVersion.ID, []domain.ModuleDependency{dependency}, nil); err != nil {
			return err
		}
		return outboxWriter.Create(txCtx, outbox.Record{
			EventType:     "protoradar.module_dependencies.updated",
			AggregateType: "module",
			AggregateID:   consumer.ID.String(),
			DedupKey:      "module:" + consumer.ID.String() + ":version:v1.0.0:dependencies-updated",
			Payload:       []byte(`{"module_id":"` + consumer.ID.String() + `"}`),
			OccurredAt:    time.Now().UTC(),
		})
	})
	if err != nil {
		t.Fatalf("transaction: %v", err)
	}
	assertTableCount(t, ctx, db, "module_dependencies", 1)
	assertTableCount(t, ctx, db, "outbox_records", 1)
}

func TestModuleDependencyRepositoryCascadeDelete(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewModuleDependencyRepository(db)
	consumer, consumerVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010018", "billing-api", "00000000-0000-0000-0000-000000020019", "v1.0.0", time.Now().UTC())
	provider, providerVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010019", "user-api", "00000000-0000-0000-0000-000000020020", "v1.0.0", time.Now().UTC())
	dependency := testModuleDependency(t, "00000000-0000-0000-0000-000000030012", consumer, consumerVersion, provider, providerVersion)
	unresolved := testUnresolvedDependency(t, "00000000-0000-0000-0000-000000040003", consumer, consumerVersion)
	if err := repo.ReplaceByConsumerModuleVersion(ctx, consumerVersion.ID, []domain.ModuleDependency{dependency}, []domain.UnresolvedProtoDependency{unresolved}); err != nil {
		t.Fatalf("replace dependencies: %v", err)
	}

	if _, err := db.executor(ctx).Exec(ctx, `DELETE FROM module_versions WHERE id = $1`, consumerVersion.ID.String()); err != nil {
		t.Fatalf("delete consumer module version: %v", err)
	}
	assertTableCount(t, ctx, db, "module_dependencies", 0)
	assertTableCount(t, ctx, db, "unresolved_proto_dependencies", 0)

	consumer, consumerVersion = createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010020", "orders-api", "00000000-0000-0000-0000-000000020021", "v1.0.0", time.Now().UTC())
	dependency = testModuleDependency(t, "00000000-0000-0000-0000-000000030013", consumer, consumerVersion, provider, providerVersion)
	if err := repo.ReplaceByConsumerModuleVersion(ctx, consumerVersion.ID, []domain.ModuleDependency{dependency}, nil); err != nil {
		t.Fatalf("replace dependencies after recreate: %v", err)
	}
	if _, err := db.executor(ctx).Exec(ctx, `DELETE FROM modules WHERE id = $1`, provider.ID.String()); err != nil {
		t.Fatalf("delete provider module: %v", err)
	}
	assertTableCount(t, ctx, db, "module_dependencies", 0)
}

func TestModuleDependencyRepositoryProviderIndexQueries(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewModuleDependencyRepository(db)
	now := time.Now().UTC()
	module, oldVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000010021", "user-api", "00000000-0000-0000-0000-000000020022", "v1.0.0", now.Add(-time.Hour))
	_, latestVersion := createGraphModuleVersionForModule(t, ctx, db, module, "00000000-0000-0000-0000-000000020023", "v1.1.0", now)
	metadataRepo := NewDescriptorMetadataRepository(db)
	if err := metadataRepo.Save(ctx, oldVersion.ID, domain.DescriptorMetadata{Files: []domain.ProtoFile{{Path: "old/v1/old.proto", PackageName: "old.v1"}}}); err != nil {
		t.Fatalf("save old metadata: %v", err)
	}
	if err := metadataRepo.Save(ctx, latestVersion.ID, domain.DescriptorMetadata{Files: []domain.ProtoFile{{
		Path:        "user/v1/user.proto",
		PackageName: "user.v1",
		Messages:    []domain.ProtoMessage{{Name: "User", FullName: "user.v1.User"}},
		Enums:       []domain.ProtoEnum{{Name: "Role", FullName: "user.v1.Role"}},
		Services:    []domain.ProtoService{{Name: "UserService", FullName: "user.v1.UserService"}},
	}}}); err != nil {
		t.Fatalf("save latest metadata: %v", err)
	}

	latest, err := repo.ListLatestPublishedModuleVersions(ctx)
	if err != nil {
		t.Fatalf("list latest versions: %v", err)
	}
	if len(latest) != 1 || latest[0].ID != latestVersion.ID {
		t.Fatalf("latest versions = %#v, want %q", latest, latestVersion.ID)
	}
	files, err := repo.ListProviderFiles(ctx, []domain.ModuleVersionID{latestVersion.ID})
	if err != nil {
		t.Fatalf("list provider files: %v", err)
	}
	if len(files) != 1 || files[0].Path != "user/v1/user.proto" || files[0].ModuleName.String() != "user-api" {
		t.Fatalf("provider files = %#v", files)
	}
	symbols, err := repo.ListProviderSymbols(ctx, []domain.ModuleVersionID{latestVersion.ID})
	if err != nil {
		t.Fatalf("list provider symbols: %v", err)
	}
	if len(symbols) != 3 {
		t.Fatalf("provider symbols = %d, want 3: %#v", len(symbols), symbols)
	}
	assertProviderSymbol(t, symbols, "user.v1.User")
	assertProviderSymbol(t, symbols, "user.v1.Role")
	assertProviderSymbol(t, symbols, "user.v1.UserService")
}

func createGraphModuleVersion(t *testing.T, ctx context.Context, db *DB, moduleID string, moduleName string, moduleVersionID string, versionValue string, createdAt time.Time) (domain.Module, domain.ModuleVersion) {
	t.Helper()
	module := createTestModule(t, ctx, db, moduleID, moduleName)
	return createGraphModuleVersionForModule(t, ctx, db, module, moduleVersionID, versionValue, createdAt)
}

func createGraphModuleVersionForModule(t *testing.T, ctx context.Context, db *DB, module domain.Module, moduleVersionID string, versionValue string, createdAt time.Time) (domain.Module, domain.ModuleVersion) {
	t.Helper()
	version, err := domain.NewVersion(versionValue)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	moduleVersion := domain.ModuleVersion{
		ID:        domain.NewModuleVersionID(moduleVersionID),
		ModuleID:  module.ID,
		Version:   version,
		Status:    domain.ModuleVersionStatusPublished,
		Digest:    "sha256:" + versionValue,
		CreatedAt: createdAt,
	}
	if err := NewModuleVersionRepository(db).Create(ctx, moduleVersion); err != nil {
		t.Fatalf("create module version: %v", err)
	}
	return module, moduleVersion
}

func testModuleDependency(t *testing.T, id string, consumer domain.Module, consumerVersion domain.ModuleVersion, provider domain.Module, providerVersion domain.ModuleVersion) domain.ModuleDependency {
	t.Helper()
	return domain.ModuleDependency{
		ID:                      domain.NewModuleDependencyID(id),
		ConsumerModuleID:        consumer.ID,
		ConsumerModuleName:      consumer.Name,
		ConsumerModuleVersionID: consumerVersion.ID,
		ConsumerVersion:         consumerVersion.Version,
		ProviderModuleID:        provider.ID,
		ProviderModuleName:      provider.Name,
		ProviderModuleVersionID: providerVersion.ID,
		ProviderVersion:         providerVersion.Version,
		Source:                  domain.DependencySourceImport,
		Reason:                  domain.DependencyResolutionReasonImportPath,
		ImportPath:              "user/v1/user.proto",
		ReferencedPackage:       "user.v1",
		ReferencedSymbol:        "",
		CreatedAt:               time.Now().UTC().Truncate(time.Microsecond),
	}
}

func testUnresolvedDependency(t *testing.T, id string, module domain.Module, version domain.ModuleVersion) domain.UnresolvedProtoDependency {
	t.Helper()
	return domain.UnresolvedProtoDependency{
		ID:                domain.NewUnresolvedProtoDependencyID(id),
		ModuleID:          module.ID,
		ModuleName:        module.Name,
		ModuleVersionID:   version.ID,
		Version:           version.Version,
		Source:            domain.DependencySourceTypeReference,
		ImportPath:        "",
		ReferencedPackage: "missing.v1",
		ReferencedSymbol:  "missing.v1.Missing",
		Reason:            domain.UnresolvedDependencyReasonNotFound,
		CreatedAt:         time.Now().UTC().Truncate(time.Microsecond),
	}
}

func assertModuleDependency(t *testing.T, got domain.ModuleDependency, want domain.ModuleDependency) {
	t.Helper()
	if got.ID != want.ID || got.ConsumerModuleID != want.ConsumerModuleID || got.ProviderModuleID != want.ProviderModuleID {
		t.Fatalf("dependency IDs = %#v, want %#v", got, want)
	}
	if got.ConsumerModuleName != want.ConsumerModuleName || got.ConsumerVersion != want.ConsumerVersion {
		t.Fatalf("consumer = %s/%s, want %s/%s", got.ConsumerModuleName, got.ConsumerVersion, want.ConsumerModuleName, want.ConsumerVersion)
	}
	if got.ProviderModuleName != want.ProviderModuleName || got.ProviderVersion != want.ProviderVersion {
		t.Fatalf("provider = %s/%s, want %s/%s", got.ProviderModuleName, got.ProviderVersion, want.ProviderModuleName, want.ProviderVersion)
	}
	if got.Source != want.Source || got.Reason != want.Reason || got.ImportPath != want.ImportPath || got.ReferencedPackage != want.ReferencedPackage || got.ReferencedSymbol != want.ReferencedSymbol {
		t.Fatalf("dependency edge = %#v, want %#v", got, want)
	}
}

func assertUnresolvedDependency(t *testing.T, got domain.UnresolvedProtoDependency, want domain.UnresolvedProtoDependency) {
	t.Helper()
	if got.ID != want.ID || got.ModuleID != want.ModuleID || got.ModuleVersionID != want.ModuleVersionID {
		t.Fatalf("unresolved IDs = %#v, want %#v", got, want)
	}
	if got.ModuleName != want.ModuleName || got.Version != want.Version {
		t.Fatalf("unresolved module = %s/%s, want %s/%s", got.ModuleName, got.Version, want.ModuleName, want.Version)
	}
	if got.Source != want.Source || got.ImportPath != want.ImportPath || got.ReferencedPackage != want.ReferencedPackage || got.ReferencedSymbol != want.ReferencedSymbol || got.Reason != want.Reason {
		t.Fatalf("unresolved edge = %#v, want %#v", got, want)
	}
}

func assertProviderSymbol(t *testing.T, symbols []registry.ProviderSymbol, fullName string) {
	t.Helper()
	for _, symbol := range symbols {
		if symbol.FullName == fullName {
			return
		}
	}
	t.Fatalf("provider symbols missing %q: %#v", fullName, symbols)
}
