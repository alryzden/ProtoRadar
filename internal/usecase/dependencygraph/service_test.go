package dependencygraph

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/integration/protoradarevents"
	"github.com/alryzden/ProtoRadar/internal/outbox"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
)

func TestRebuildResolvesImportPathToProviderModule(t *testing.T) {
	fixture := newFixture(t)
	fixture.metadata.byVersion[fixture.consumerVersion.ID.String()] = domain.DescriptorMetadata{Files: []domain.ProtoFile{{
		Path:    "billing/v1/billing.proto",
		Imports: []domain.ProtoImport{{Path: "user/v1/user.proto"}},
	}}}
	fixture.providers.files = []registry.ProviderFile{providerFile(fixture.provider, fixture.providerVersion, "user/v1/user.proto")}

	output, err := fixture.service.RebuildModuleVersionDependencies(context.Background(), RebuildModuleVersionDependenciesInput{ModuleID: fixture.consumer.ID, ModuleVersionID: fixture.consumerVersion.ID})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if len(output.Dependencies) != 1 {
		t.Fatalf("dependencies = %d, want 1", len(output.Dependencies))
	}
	dependency := output.Dependencies[0]
	if dependency.ProviderModuleID != fixture.provider.ID || dependency.Source != domain.DependencySourceImport || dependency.Reason != domain.DependencyResolutionReasonImportPath || dependency.ImportPath != "user/v1/user.proto" {
		t.Fatalf("dependency = %#v", dependency)
	}
	if len(output.Unresolved) != 0 {
		t.Fatalf("unresolved = %#v", output.Unresolved)
	}
}

func TestRebuildResolvesFieldTypeReferenceToProviderModule(t *testing.T) {
	fixture := newFixture(t)
	fixture.metadata.byVersion[fixture.consumerVersion.ID.String()] = domain.DescriptorMetadata{Files: []domain.ProtoFile{{
		Path: "billing/v1/billing.proto",
		Messages: []domain.ProtoMessage{{
			Name:     "Invoice",
			FullName: "billing.v1.Invoice",
			Fields:   []domain.ProtoField{{Name: "user", Number: 1, TypeName: ".user.v1.User"}},
		}},
	}}}
	fixture.providers.symbols = []registry.ProviderSymbol{providerSymbol(fixture.provider, fixture.providerVersion, "user.v1", "user.v1.User")}

	output, err := fixture.service.RebuildModuleVersionDependencies(context.Background(), RebuildModuleVersionDependenciesInput{ModuleName: "billing-api", Version: "v1.0.0"})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	assertSingleSymbolDependency(t, output.Dependencies, domain.DependencySourceTypeReference, "user.v1.User")
}

func TestRebuildResolvesMethodInputTypeToProviderModule(t *testing.T) {
	fixture := newFixture(t)
	fixture.metadata.byVersion[fixture.consumerVersion.ID.String()] = methodMetadata(".user.v1.GetUserRequest", "")
	fixture.providers.symbols = []registry.ProviderSymbol{providerSymbol(fixture.provider, fixture.providerVersion, "user.v1", "user.v1.GetUserRequest")}

	output, err := fixture.service.RebuildModuleVersionDependencies(context.Background(), RebuildModuleVersionDependenciesInput{ModuleID: fixture.consumer.ID, ModuleVersionID: fixture.consumerVersion.ID})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	assertSingleSymbolDependency(t, output.Dependencies, domain.DependencySourceMethodReference, "user.v1.GetUserRequest")
}

func TestRebuildResolvesMethodOutputTypeToProviderModule(t *testing.T) {
	fixture := newFixture(t)
	fixture.metadata.byVersion[fixture.consumerVersion.ID.String()] = methodMetadata("", ".user.v1.User")
	fixture.providers.symbols = []registry.ProviderSymbol{providerSymbol(fixture.provider, fixture.providerVersion, "user.v1", "user.v1.User")}

	output, err := fixture.service.RebuildModuleVersionDependencies(context.Background(), RebuildModuleVersionDependenciesInput{ModuleID: fixture.consumer.ID, ModuleVersionID: fixture.consumerVersion.ID})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	assertSingleSymbolDependency(t, output.Dependencies, domain.DependencySourceMethodReference, "user.v1.User")
}

func TestRebuildIgnoresSelfDependencies(t *testing.T) {
	fixture := newFixture(t)
	fixture.metadata.byVersion[fixture.consumerVersion.ID.String()] = domain.DescriptorMetadata{Files: []domain.ProtoFile{
		{Path: "billing/v1/common.proto", PackageName: "billing.v1", Messages: []domain.ProtoMessage{{Name: "Money", FullName: "billing.v1.Money"}}},
		{Path: "billing/v1/billing.proto", Imports: []domain.ProtoImport{{Path: "billing/v1/common.proto"}}, Messages: []domain.ProtoMessage{{Name: "Invoice", FullName: "billing.v1.Invoice", Fields: []domain.ProtoField{{Name: "money", Number: 1, TypeName: ".billing.v1.Money"}}}}},
	}}

	output, err := fixture.service.RebuildModuleVersionDependencies(context.Background(), RebuildModuleVersionDependenciesInput{ModuleID: fixture.consumer.ID, ModuleVersionID: fixture.consumerVersion.ID})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if len(output.Dependencies) != 0 || len(output.Unresolved) != 0 {
		t.Fatalf("self refs should be ignored, dependencies=%#v unresolved=%#v", output.Dependencies, output.Unresolved)
	}
}

func TestRebuildIgnoresGoogleProtobufWellKnownImports(t *testing.T) {
	fixture := newFixture(t)
	fixture.metadata.byVersion[fixture.consumerVersion.ID.String()] = domain.DescriptorMetadata{Files: []domain.ProtoFile{{
		Path:    "billing/v1/billing.proto",
		Imports: []domain.ProtoImport{{Path: "google/protobuf/timestamp.proto"}},
		Messages: []domain.ProtoMessage{{
			Name:     "Invoice",
			FullName: "billing.v1.Invoice",
			Fields:   []domain.ProtoField{{Name: "created_at", Number: 1, TypeName: ".google.protobuf.Timestamp"}},
		}},
	}}}

	output, err := fixture.service.RebuildModuleVersionDependencies(context.Background(), RebuildModuleVersionDependenciesInput{ModuleID: fixture.consumer.ID, ModuleVersionID: fixture.consumerVersion.ID})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if len(output.Dependencies) != 0 || len(output.Unresolved) != 0 {
		t.Fatalf("well-known refs should be ignored, dependencies=%#v unresolved=%#v", output.Dependencies, output.Unresolved)
	}
}

func TestRebuildRecordsUnresolvedImportWhenProviderNotFound(t *testing.T) {
	fixture := newFixture(t)
	fixture.metadata.byVersion[fixture.consumerVersion.ID.String()] = domain.DescriptorMetadata{Files: []domain.ProtoFile{{Path: "billing.proto", Imports: []domain.ProtoImport{{Path: "missing/v1/missing.proto"}}}}}

	output, err := fixture.service.RebuildModuleVersionDependencies(context.Background(), RebuildModuleVersionDependenciesInput{ModuleID: fixture.consumer.ID, ModuleVersionID: fixture.consumerVersion.ID})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	assertSingleUnresolved(t, output.Unresolved, domain.DependencySourceImport, "missing/v1/missing.proto", "", domain.UnresolvedDependencyReasonProviderNotFound)
}

func TestRebuildRecordsUnresolvedTypeReferenceWhenSymbolNotFound(t *testing.T) {
	fixture := newFixture(t)
	fixture.metadata.byVersion[fixture.consumerVersion.ID.String()] = domain.DescriptorMetadata{Files: []domain.ProtoFile{{Messages: []domain.ProtoMessage{{Name: "Invoice", FullName: "billing.v1.Invoice", Fields: []domain.ProtoField{{Name: "missing", Number: 1, TypeName: ".missing.v1.Missing"}}}}}}}

	output, err := fixture.service.RebuildModuleVersionDependencies(context.Background(), RebuildModuleVersionDependenciesInput{ModuleID: fixture.consumer.ID, ModuleVersionID: fixture.consumerVersion.ID})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	assertSingleUnresolved(t, output.Unresolved, domain.DependencySourceTypeReference, "", "missing.v1.Missing", domain.UnresolvedDependencyReasonProviderNotFound)
}

func TestRebuildRecordsAmbiguousProviderAsUnresolved(t *testing.T) {
	fixture := newFixture(t)
	otherProvider, otherProviderVersion := testModuleVersion(t, "module-3", "profile-api", "module-version-3", "v1.0.0")
	fixture.modules.add(otherProvider)
	fixture.versions.add(otherProviderVersion)
	fixture.providers.latest = append(fixture.providers.latest, otherProviderVersion)
	fixture.metadata.byVersion[fixture.consumerVersion.ID.String()] = domain.DescriptorMetadata{Files: []domain.ProtoFile{{Path: "billing.proto", Imports: []domain.ProtoImport{{Path: "shared/v1/shared.proto"}}}}}
	fixture.providers.files = []registry.ProviderFile{
		providerFile(fixture.provider, fixture.providerVersion, "shared/v1/shared.proto"),
		providerFile(otherProvider, otherProviderVersion, "shared/v1/shared.proto"),
	}

	output, err := fixture.service.RebuildModuleVersionDependencies(context.Background(), RebuildModuleVersionDependenciesInput{ModuleID: fixture.consumer.ID, ModuleVersionID: fixture.consumerVersion.ID})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	assertSingleUnresolved(t, output.Unresolved, domain.DependencySourceImport, "shared/v1/shared.proto", "", domain.UnresolvedDependencyReasonAmbiguousProvider)
}

func TestRebuildDeduplicatesEdges(t *testing.T) {
	fixture := newFixture(t)
	fixture.metadata.byVersion[fixture.consumerVersion.ID.String()] = domain.DescriptorMetadata{Files: []domain.ProtoFile{{
		Path: "billing.proto",
		Messages: []domain.ProtoMessage{{
			Name:     "Invoice",
			FullName: "billing.v1.Invoice",
			Fields: []domain.ProtoField{
				{Name: "user", Number: 1, TypeName: ".user.v1.User"},
				{Name: "owner", Number: 2, TypeName: ".user.v1.User"},
			},
		}},
	}}}
	fixture.providers.symbols = []registry.ProviderSymbol{providerSymbol(fixture.provider, fixture.providerVersion, "user.v1", "user.v1.User")}

	output, err := fixture.service.RebuildModuleVersionDependencies(context.Background(), RebuildModuleVersionDependenciesInput{ModuleID: fixture.consumer.ID, ModuleVersionID: fixture.consumerVersion.ID})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if len(output.Dependencies) != 1 {
		t.Fatalf("dependencies = %d, want deduped 1: %#v", len(output.Dependencies), output.Dependencies)
	}
}

func TestRebuildReplacesOldDependencies(t *testing.T) {
	fixture := newFixture(t)
	fixture.dependencies.dependencies = []domain.ModuleDependency{{ID: domain.NewModuleDependencyID("old")}}
	fixture.metadata.byVersion[fixture.consumerVersion.ID.String()] = domain.DescriptorMetadata{Files: []domain.ProtoFile{{Path: "billing.proto", Imports: []domain.ProtoImport{{Path: "user/v1/user.proto"}}}}}
	fixture.providers.files = []registry.ProviderFile{providerFile(fixture.provider, fixture.providerVersion, "user/v1/user.proto")}

	if _, err := fixture.service.RebuildModuleVersionDependencies(context.Background(), RebuildModuleVersionDependenciesInput{ModuleID: fixture.consumer.ID, ModuleVersionID: fixture.consumerVersion.ID}); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if !fixture.dependencies.replaced || len(fixture.dependencies.dependencies) != 1 || fixture.dependencies.dependencies[0].ID.String() == "old" {
		t.Fatalf("dependencies were not replaced: %#v", fixture.dependencies.dependencies)
	}
}

func TestRebuildWritesModuleDependenciesUpdatedOutboxTransactionally(t *testing.T) {
	fixture := newFixture(t)
	fixture.metadata.byVersion[fixture.consumerVersion.ID.String()] = domain.DescriptorMetadata{Files: []domain.ProtoFile{{Path: "billing.proto", Imports: []domain.ProtoImport{{Path: "user/v1/user.proto"}}}}}
	fixture.providers.files = []registry.ProviderFile{providerFile(fixture.provider, fixture.providerVersion, "user/v1/user.proto")}

	if _, err := fixture.service.RebuildModuleVersionDependencies(context.Background(), RebuildModuleVersionDependenciesInput{ModuleID: fixture.consumer.ID, ModuleVersionID: fixture.consumerVersion.ID}); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if len(fixture.outbox.records) != 1 {
		t.Fatalf("outbox records = %d, want 1", len(fixture.outbox.records))
	}
	record := fixture.outbox.records[0]
	if record.EventType != protoradarevents.EventTypeModuleDependenciesUpdated {
		t.Fatalf("event type = %q", record.EventType)
	}
	if record.DedupKey != "module:module-1:version:v1.0.0:dependencies-updated" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}
	var payload protoradarevents.ModuleDependenciesUpdatedPayload
	if err := json.Unmarshal(record.Payload, &payload); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	if payload.DependencyCount != 1 || payload.UnresolvedDependencyCount != 0 {
		t.Fatalf("payload counts = %#v", payload)
	}
}

func TestRebuildRollbackPreventsDependenciesAndOutboxRecord(t *testing.T) {
	fixture := newFixture(t)
	fixture.outbox.createErr = errors.New("outbox failed")
	fixture.metadata.byVersion[fixture.consumerVersion.ID.String()] = domain.DescriptorMetadata{Files: []domain.ProtoFile{{Path: "billing.proto", Imports: []domain.ProtoImport{{Path: "user/v1/user.proto"}}}}}
	fixture.providers.files = []registry.ProviderFile{providerFile(fixture.provider, fixture.providerVersion, "user/v1/user.proto")}

	_, err := fixture.service.RebuildModuleVersionDependencies(context.Background(), RebuildModuleVersionDependenciesInput{ModuleID: fixture.consumer.ID, ModuleVersionID: fixture.consumerVersion.ID})
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(fixture.dependencies.dependencies) != 0 || len(fixture.outbox.records) != 0 {
		t.Fatalf("rollback failed, dependencies=%#v outbox=%#v", fixture.dependencies.dependencies, fixture.outbox.records)
	}
}

func TestRebuildDirectDependenciesOnly(t *testing.T) {
	fixture := newFixture(t)
	transitiveProvider, transitiveProviderVersion := testModuleVersion(t, "module-4", "common-api", "module-version-4", "v1.0.0")
	fixture.modules.add(transitiveProvider)
	fixture.versions.add(transitiveProviderVersion)
	fixture.providers.latest = append(fixture.providers.latest, transitiveProviderVersion)
	fixture.metadata.byVersion[fixture.consumerVersion.ID.String()] = domain.DescriptorMetadata{Files: []domain.ProtoFile{{Path: "billing.proto", Imports: []domain.ProtoImport{{Path: "user/v1/user.proto"}}}}}
	fixture.providers.files = []registry.ProviderFile{
		providerFile(fixture.provider, fixture.providerVersion, "user/v1/user.proto"),
		providerFile(transitiveProvider, transitiveProviderVersion, "common/v1/common.proto"),
	}
	fixture.providers.symbols = []registry.ProviderSymbol{providerSymbol(transitiveProvider, transitiveProviderVersion, "common.v1", "common.v1.Money")}

	output, err := fixture.service.RebuildModuleVersionDependencies(context.Background(), RebuildModuleVersionDependenciesInput{ModuleID: fixture.consumer.ID, ModuleVersionID: fixture.consumerVersion.ID})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if len(output.Dependencies) != 1 || output.Dependencies[0].ProviderModuleID != fixture.provider.ID {
		t.Fatalf("expected only direct user-api dependency, got %#v", output.Dependencies)
	}
}

func methodMetadata(inputType string, outputType string) domain.DescriptorMetadata {
	return domain.DescriptorMetadata{Files: []domain.ProtoFile{{
		Path: "billing/v1/billing.proto",
		Services: []domain.ProtoService{{
			Name:     "BillingService",
			FullName: "billing.v1.BillingService",
			Methods:  []domain.ProtoMethod{{Name: "GetInvoice", InputType: inputType, OutputType: outputType}},
		}},
	}}}
}

func assertSingleSymbolDependency(t *testing.T, dependencies []domain.ModuleDependency, source domain.DependencySource, symbol string) {
	t.Helper()
	if len(dependencies) != 1 {
		t.Fatalf("dependencies = %d, want 1: %#v", len(dependencies), dependencies)
	}
	dependency := dependencies[0]
	if dependency.Source != source || dependency.Reason != domain.DependencyResolutionReasonSymbol || dependency.ReferencedSymbol != symbol || dependency.ProviderModuleName.String() != "user-api" {
		t.Fatalf("dependency = %#v", dependency)
	}
}

func assertSingleUnresolved(t *testing.T, unresolved []domain.UnresolvedProtoDependency, source domain.DependencySource, importPath string, symbol string, reason domain.UnresolvedDependencyReason) {
	t.Helper()
	if len(unresolved) != 1 {
		t.Fatalf("unresolved = %d, want 1: %#v", len(unresolved), unresolved)
	}
	item := unresolved[0]
	if item.Source != source || item.ImportPath != importPath || item.ReferencedSymbol != symbol || item.Reason != reason {
		t.Fatalf("unresolved = %#v", item)
	}
}

type fixture struct {
	service         *Service
	modules         *fakeModules
	versions        *fakeVersions
	metadata        *fakeMetadata
	dependencies    *fakeDependencies
	providers       *fakeProviders
	transactions    *fakeTransactions
	outbox          *fakeOutbox
	clock           *fakeClock
	ids             *fakeIDs
	consumer        domain.Module
	consumerVersion domain.ModuleVersion
	provider        domain.Module
	providerVersion domain.ModuleVersion
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	consumer, consumerVersion := testModuleVersion(t, "module-1", "billing-api", "module-version-1", "v1.0.0")
	provider, providerVersion := testModuleVersion(t, "module-2", "user-api", "module-version-2", "v1.0.0")
	modules := newFakeModules(consumer, provider)
	versions := newFakeVersions(consumerVersion, providerVersion)
	metadata := newFakeMetadata()
	dependencies := &fakeDependencies{}
	providers := &fakeProviders{latest: []domain.ModuleVersion{consumerVersion, providerVersion}}
	outboxWriter := &fakeOutbox{}
	transactions := &fakeTransactions{dependencies: dependencies, outbox: outboxWriter}
	clock := &fakeClock{now: time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)}
	ids := &fakeIDs{}
	return &fixture{
		service: NewService(
			modules,
			versions,
			metadata,
			dependencies,
			providers,
			transactions,
			outboxWriter,
			clock,
			ids,
		),
		modules:         modules,
		versions:        versions,
		metadata:        metadata,
		dependencies:    dependencies,
		providers:       providers,
		transactions:    transactions,
		outbox:          outboxWriter,
		clock:           clock,
		ids:             ids,
		consumer:        consumer,
		consumerVersion: consumerVersion,
		provider:        provider,
		providerVersion: providerVersion,
	}
}

func testModuleVersion(t *testing.T, moduleID string, moduleNameValue string, moduleVersionID string, versionValue string) (domain.Module, domain.ModuleVersion) {
	t.Helper()
	moduleName, err := domain.NewModuleName(moduleNameValue)
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	version, err := domain.NewVersion(versionValue)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	module := domain.Module{ID: domain.NewModuleID(moduleID), Name: moduleName}
	moduleVersion := domain.ModuleVersion{ID: domain.NewModuleVersionID(moduleVersionID), ModuleID: module.ID, Version: version, Status: domain.ModuleVersionStatusPublished, CreatedAt: time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)}
	return module, moduleVersion
}

func providerFile(module domain.Module, moduleVersion domain.ModuleVersion, path string) registry.ProviderFile {
	return registry.ProviderFile{Path: path, ModuleID: module.ID, ModuleName: module.Name, ModuleVersionID: moduleVersion.ID, Version: moduleVersion.Version}
}

func providerSymbol(module domain.Module, moduleVersion domain.ModuleVersion, packageName string, fullName string) registry.ProviderSymbol {
	return registry.ProviderSymbol{FullName: fullName, PackageName: packageName, ModuleID: module.ID, ModuleName: module.Name, ModuleVersionID: moduleVersion.ID, Version: moduleVersion.Version}
}

type fakeModules struct {
	byID   map[string]domain.Module
	byName map[string]domain.Module
}

func newFakeModules(modules ...domain.Module) *fakeModules {
	repo := &fakeModules{byID: map[string]domain.Module{}, byName: map[string]domain.Module{}}
	for _, module := range modules {
		repo.add(module)
	}
	return repo
}

func (repo *fakeModules) add(module domain.Module) {
	repo.byID[module.ID.String()] = module
	repo.byName[module.Name.String()] = module
}

func (repo *fakeModules) Create(ctx context.Context, module domain.Module) error { return nil }
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
	return nil, nil
}

type fakeVersions struct {
	byID            map[string]domain.ModuleVersion
	byModuleVersion map[string]domain.ModuleVersion
}

func newFakeVersions(versions ...domain.ModuleVersion) *fakeVersions {
	repo := &fakeVersions{byID: map[string]domain.ModuleVersion{}, byModuleVersion: map[string]domain.ModuleVersion{}}
	for _, version := range versions {
		repo.add(version)
	}
	return repo
}

func (repo *fakeVersions) add(version domain.ModuleVersion) {
	repo.byID[version.ID.String()] = version
	repo.byModuleVersion[version.ModuleID.String()+":"+version.Version.String()] = version
}

func (repo *fakeVersions) Create(ctx context.Context, version domain.ModuleVersion) error { return nil }
func (repo *fakeVersions) GetByID(ctx context.Context, id domain.ModuleVersionID) (domain.ModuleVersion, error) {
	version, exists := repo.byID[id.String()]
	if !exists {
		return domain.ModuleVersion{}, domain.ErrNotFound
	}
	return version, nil
}
func (repo *fakeVersions) GetByModuleAndVersion(ctx context.Context, moduleID domain.ModuleID, version domain.Version) (domain.ModuleVersion, error) {
	moduleVersion, exists := repo.byModuleVersion[moduleID.String()+":"+version.String()]
	if !exists {
		return domain.ModuleVersion{}, domain.ErrNotFound
	}
	return moduleVersion, nil
}
func (repo *fakeVersions) GetLatestByModule(ctx context.Context, moduleID domain.ModuleID) (domain.ModuleVersion, error) {
	return domain.ModuleVersion{}, nil
}
func (repo *fakeVersions) ListByModule(ctx context.Context, moduleID domain.ModuleID, limit int, offset int) ([]domain.ModuleVersion, error) {
	return nil, nil
}

type fakeMetadata struct {
	byVersion map[string]domain.DescriptorMetadata
	err       error
}

func newFakeMetadata() *fakeMetadata {
	return &fakeMetadata{byVersion: map[string]domain.DescriptorMetadata{}}
}

func (repo *fakeMetadata) Save(ctx context.Context, moduleVersionID domain.ModuleVersionID, metadata domain.DescriptorMetadata) error {
	return nil
}
func (repo *fakeMetadata) GetByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) (domain.DescriptorMetadata, error) {
	if repo.err != nil {
		return domain.DescriptorMetadata{}, repo.err
	}
	metadata, exists := repo.byVersion[moduleVersionID.String()]
	if !exists {
		return domain.DescriptorMetadata{}, domain.ErrNotFound
	}
	return metadata, nil
}
func (repo *fakeMetadata) GetSummaryByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) (domain.DescriptorMetadataSummary, error) {
	return domain.DescriptorMetadataSummary{}, nil
}

type fakeDependencies struct {
	dependencies []domain.ModuleDependency
	unresolved   []domain.UnresolvedProtoDependency
	replaced     bool
	err          error
}

func (repo *fakeDependencies) ReplaceByConsumerModuleVersion(ctx context.Context, consumerModuleVersionID domain.ModuleVersionID, dependencies []domain.ModuleDependency, unresolved []domain.UnresolvedProtoDependency) error {
	if repo.err != nil {
		return repo.err
	}
	repo.replaced = true
	repo.dependencies = slices.Clone(dependencies)
	repo.unresolved = slices.Clone(unresolved)
	return nil
}
func (repo *fakeDependencies) ListUpstreamByModule(ctx context.Context, moduleID domain.ModuleID) ([]domain.ModuleDependency, error) {
	return nil, nil
}
func (repo *fakeDependencies) ListUpstreamByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) ([]domain.ModuleDependency, error) {
	return nil, nil
}
func (repo *fakeDependencies) ListDownstreamByModule(ctx context.Context, moduleID domain.ModuleID) ([]domain.ModuleDependency, error) {
	return nil, nil
}
func (repo *fakeDependencies) ListAffectedModules(ctx context.Context, providerModuleID domain.ModuleID) ([]domain.AffectedModule, error) {
	return nil, nil
}
func (repo *fakeDependencies) ListUnresolvedByModule(ctx context.Context, moduleID domain.ModuleID) ([]domain.UnresolvedProtoDependency, error) {
	return nil, nil
}
func (repo *fakeDependencies) ListUnresolvedByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) ([]domain.UnresolvedProtoDependency, error) {
	return nil, nil
}

func (repo *fakeDependencies) snapshot() *fakeDependencies {
	return &fakeDependencies{dependencies: slices.Clone(repo.dependencies), unresolved: slices.Clone(repo.unresolved), replaced: repo.replaced, err: repo.err}
}

func (repo *fakeDependencies) restore(snapshot *fakeDependencies) {
	repo.dependencies = snapshot.dependencies
	repo.unresolved = snapshot.unresolved
	repo.replaced = snapshot.replaced
	repo.err = snapshot.err
}

type fakeProviders struct {
	latest  []domain.ModuleVersion
	files   []registry.ProviderFile
	symbols []registry.ProviderSymbol
	err     error
}

func (repo *fakeProviders) ListLatestPublishedModuleVersions(ctx context.Context) ([]domain.ModuleVersion, error) {
	if repo.err != nil {
		return nil, repo.err
	}
	return slices.Clone(repo.latest), nil
}
func (repo *fakeProviders) ListProviderFiles(ctx context.Context, moduleVersionIDs []domain.ModuleVersionID) ([]registry.ProviderFile, error) {
	if repo.err != nil {
		return nil, repo.err
	}
	allowed := moduleVersionSet(moduleVersionIDs)
	items := make([]registry.ProviderFile, 0)
	for _, file := range repo.files {
		if allowed[file.ModuleVersionID.String()] {
			items = append(items, file)
		}
	}
	return items, nil
}
func (repo *fakeProviders) ListProviderSymbols(ctx context.Context, moduleVersionIDs []domain.ModuleVersionID) ([]registry.ProviderSymbol, error) {
	if repo.err != nil {
		return nil, repo.err
	}
	allowed := moduleVersionSet(moduleVersionIDs)
	items := make([]registry.ProviderSymbol, 0)
	for _, symbol := range repo.symbols {
		if allowed[symbol.ModuleVersionID.String()] {
			items = append(items, symbol)
		}
	}
	return items, nil
}

func moduleVersionSet(ids []domain.ModuleVersionID) map[string]bool {
	set := map[string]bool{}
	for _, id := range ids {
		set[id.String()] = true
	}
	return set
}

type fakeTransactions struct {
	dependencies *fakeDependencies
	outbox       *fakeOutbox
}

func (tx *fakeTransactions) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	dependencies := tx.dependencies.snapshot()
	outboxRecords := slices.Clone(tx.outbox.records)
	if err := fn(ctx); err != nil {
		tx.dependencies.restore(dependencies)
		tx.outbox.records = outboxRecords
		return err
	}
	return nil
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

type fakeClock struct {
	now time.Time
}

func (clock *fakeClock) Now() time.Time {
	return clock.now
}

type fakeIDs struct {
	dependencyID int
	unresolvedID int
}

func (ids *fakeIDs) NewModuleDependencyID() (domain.ModuleDependencyID, error) {
	ids.dependencyID++
	return domain.NewModuleDependencyID("dependency-" + strconv.Itoa(ids.dependencyID)), nil
}

func (ids *fakeIDs) NewUnresolvedProtoDependencyID() (domain.UnresolvedProtoDependencyID, error) {
	ids.unresolvedID++
	return domain.NewUnresolvedProtoDependencyID("unresolved-" + strconv.Itoa(ids.unresolvedID)), nil
}
