package runtimeinventory

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/integration/protoradarevents"
	"github.com/alryzden/ProtoRadar/internal/outbox"
)

func TestReportRuntimeInventoryCreatesServiceDeploymentAndUsages(t *testing.T) {
	fixture := newFixture(t)
	fixture.addModuleVersion(t, "user-api", "v1.0.0", "module-1", "version-1", fixture.clock.now)

	output, err := fixture.service.ReportRuntimeInventory(context.Background(), ReportRuntimeInventoryInput{
		ServiceName:  "billing-service",
		Environment:  "prod",
		GitCommit:    "abc123",
		BuildVersion: "pipeline-1",
		Modules:      []ReportedModuleInput{{Module: "user-api", Version: "v1.0.0"}},
	})
	if err != nil {
		t.Fatalf("report runtime inventory: %v", err)
	}

	if output.DeploymentID != "deployment-1" || output.ServiceName != "billing-service" || output.Environment != "prod" {
		t.Fatalf("output = %#v", output)
	}
	if len(output.Usages) != 1 || output.Usages[0].DriftStatus != domain.RuntimeDriftStatusUpToDate {
		t.Fatalf("usages = %#v", output.Usages)
	}
	if len(fixture.runtime.servicesByName) != 1 || len(fixture.runtime.deployments) != 1 || len(fixture.runtime.usages) != 1 {
		t.Fatalf("stored services/deployments/usages = %d/%d/%d", len(fixture.runtime.servicesByName), len(fixture.runtime.deployments), len(fixture.runtime.usages))
	}
	if fixture.runtime.deployments[0].ID.String() != "deployment-1" || fixture.runtime.deployments[0].ServiceID.String() != "service-1" {
		t.Fatalf("deployment = %#v", fixture.runtime.deployments[0])
	}
}

func TestReportRuntimeInventoryDeduplicatesModuleUsages(t *testing.T) {
	fixture := newFixture(t)
	fixture.addModuleVersion(t, "user-api", "v1.0.0", "module-1", "version-1", fixture.clock.now)

	output, err := fixture.service.ReportRuntimeInventory(context.Background(), validReport([]ReportedModuleInput{
		{Module: "user-api", Version: "v1.0.0"},
		{Module: "user-api", Version: "v1.0.0"},
	}))
	if err != nil {
		t.Fatalf("report runtime inventory: %v", err)
	}
	if len(output.Usages) != 1 || len(fixture.runtime.usages) != 1 {
		t.Fatalf("deduplicated usages output/stored = %d/%d", len(output.Usages), len(fixture.runtime.usages))
	}
}

func TestReportRuntimeInventoryKnownLatestVersionIsUpToDate(t *testing.T) {
	fixture := newFixture(t)
	fixture.addModuleVersion(t, "user-api", "v1.0.0", "module-1", "version-1", fixture.clock.now)

	output, err := fixture.service.ReportRuntimeInventory(context.Background(), validReport([]ReportedModuleInput{{Module: "user-api", Version: "v1.0.0"}}))
	if err != nil {
		t.Fatalf("report runtime inventory: %v", err)
	}
	usage := output.Usages[0]
	if usage.DriftStatus != domain.RuntimeDriftStatusUpToDate || usage.DriftReason != DriftReasonUpToDate || usage.LatestVersion != "v1.0.0" {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestReportRuntimeInventoryKnownOlderVersionIsBehindLatest(t *testing.T) {
	fixture := newFixture(t)
	fixture.addModuleVersion(t, "user-api", "v1.0.0", "module-1", "version-1", fixture.clock.now.Add(-time.Hour))
	fixture.addModuleVersion(t, "user-api", "v1.1.0", "module-1", "version-2", fixture.clock.now)

	output, err := fixture.service.ReportRuntimeInventory(context.Background(), validReport([]ReportedModuleInput{{Module: "user-api", Version: "v1.0.0"}}))
	if err != nil {
		t.Fatalf("report runtime inventory: %v", err)
	}
	usage := output.Usages[0]
	if usage.DriftStatus != domain.RuntimeDriftStatusBehindLatest || usage.LatestVersion != "v1.1.0" {
		t.Fatalf("usage = %#v", usage)
	}
	if !strings.Contains(usage.DriftReason, "v1.1.0") {
		t.Fatalf("drift reason %q should include latest version", usage.DriftReason)
	}
}

func TestReportRuntimeInventoryUnknownModuleIsUnknownVersion(t *testing.T) {
	fixture := newFixture(t)

	output, err := fixture.service.ReportRuntimeInventory(context.Background(), validReport([]ReportedModuleInput{{Module: "missing-api", Version: "v1.0.0"}}))
	if err != nil {
		t.Fatalf("report runtime inventory: %v", err)
	}
	usage := output.Usages[0]
	if usage.DriftStatus != domain.RuntimeDriftStatusUnknownVersion || usage.DriftReason != DriftReasonModuleNotFound {
		t.Fatalf("usage = %#v", usage)
	}
	if fixture.runtime.usages[0].ModuleID != nil || fixture.runtime.usages[0].ModuleVersionID != nil {
		t.Fatalf("unknown module IDs should be nil: %#v", fixture.runtime.usages[0])
	}
}

func TestReportRuntimeInventoryUnknownVersionIsUnknownVersion(t *testing.T) {
	fixture := newFixture(t)
	fixture.addModule(t, "user-api", "module-1")

	output, err := fixture.service.ReportRuntimeInventory(context.Background(), validReport([]ReportedModuleInput{{Module: "user-api", Version: "v9.9.9"}}))
	if err != nil {
		t.Fatalf("report runtime inventory: %v", err)
	}
	usage := output.Usages[0]
	if usage.DriftStatus != domain.RuntimeDriftStatusUnknownVersion || usage.DriftReason != DriftReasonVersionNotFound {
		t.Fatalf("usage = %#v", usage)
	}
	if fixture.runtime.usages[0].ModuleID == nil || fixture.runtime.usages[0].ModuleVersionID != nil {
		t.Fatalf("unknown version should keep module id and nil version id: %#v", fixture.runtime.usages[0])
	}
}

func TestReportRuntimeInventoryWritesOutboxRecordTransactionally(t *testing.T) {
	fixture := newFixture(t)
	fixture.addModuleVersion(t, "user-api", "v1.0.0", "module-1", "version-1", fixture.clock.now)

	output, err := fixture.service.ReportRuntimeInventory(context.Background(), validReport([]ReportedModuleInput{{Module: "user-api", Version: "v1.0.0"}}))
	if err != nil {
		t.Fatalf("report runtime inventory: %v", err)
	}
	if !fixture.transactions.inTransactionAtRuntimeWrite || !fixture.transactions.inTransactionAtOutboxWrite {
		t.Fatalf("runtime and outbox writes should happen inside transaction")
	}
	if len(fixture.outbox.records) != 1 {
		t.Fatalf("outbox records = %d, want 1", len(fixture.outbox.records))
	}
	record := fixture.outbox.records[0]
	if record.EventType != protoradarevents.EventTypeRuntimeInventoryReported {
		t.Fatalf("event type = %q", record.EventType)
	}
	if record.DedupKey != "runtime-deployment:"+output.DeploymentID+":reported" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}
}

func TestReportRuntimeInventoryRollbackPreventsServiceDeploymentUsagesAndOutbox(t *testing.T) {
	fixture := newFixture(t)
	fixture.addModuleVersion(t, "user-api", "v1.0.0", "module-1", "version-1", fixture.clock.now)
	fixture.outbox.createErr = errors.New("outbox failed")

	_, err := fixture.service.ReportRuntimeInventory(context.Background(), validReport([]ReportedModuleInput{{Module: "user-api", Version: "v1.0.0"}}))
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(fixture.runtime.servicesByName) != 0 || len(fixture.runtime.deployments) != 0 || len(fixture.runtime.usages) != 0 || len(fixture.outbox.records) != 0 {
		t.Fatalf("rollback failed: services/deployments/usages/outbox = %d/%d/%d/%d", len(fixture.runtime.servicesByName), len(fixture.runtime.deployments), len(fixture.runtime.usages), len(fixture.outbox.records))
	}
}

func TestReportRuntimeInventoryDriftCountsReturnedAndIncludedInEvent(t *testing.T) {
	fixture := newFixture(t)
	fixture.addModuleVersion(t, "user-api", "v1.0.0", "module-1", "version-1", fixture.clock.now.Add(-time.Hour))
	fixture.addModuleVersion(t, "user-api", "v1.1.0", "module-1", "version-2", fixture.clock.now)
	fixture.addModule(t, "account-api", "module-2")

	output, err := fixture.service.ReportRuntimeInventory(context.Background(), validReport([]ReportedModuleInput{
		{Module: "user-api", Version: "v1.1.0"},
		{Module: "user-api", Version: "v1.0.0"},
		{Module: "missing-api", Version: "v1.0.0"},
		{Module: "account-api", Version: "v9.9.9"},
	}))
	if err != nil {
		t.Fatalf("report runtime inventory: %v", err)
	}
	if output.DriftCounts.UpToDate != 1 || output.DriftCounts.BehindLatest != 1 || output.DriftCounts.UnknownVersion != 2 {
		t.Fatalf("drift counts = %#v", output.DriftCounts)
	}
	var payload protoradarevents.RuntimeInventoryReportedPayload
	if err := json.Unmarshal(fixture.outbox.records[0].Payload, &payload); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	if payload.DriftCounts.UpToDate != 1 || payload.DriftCounts.BehindLatest != 1 || payload.DriftCounts.UnknownVersion != 2 || payload.ModuleUsageCount != 4 {
		t.Fatalf("event payload = %#v", payload)
	}
}

func TestReportRuntimeInventoryRejectsEmptyModules(t *testing.T) {
	fixture := newFixture(t)
	_, err := fixture.service.ReportRuntimeInventory(context.Background(), validReport(nil))
	if !errors.Is(err, ErrRuntimeModulesRequired) {
		t.Fatalf("error = %v, want ErrRuntimeModulesRequired", err)
	}
}

func TestListRuntimeServicesReturnsSummaries(t *testing.T) {
	fixture := newFixture(t)
	summary := testRuntimeServiceSummary(t, "billing-service")
	fixture.runtime.summaries = []domain.RuntimeServiceSummary{summary}

	items, err := fixture.service.ListRuntimeServices(context.Background(), 20, 0)
	if err != nil {
		t.Fatalf("list runtime services: %v", err)
	}
	if len(items) != 1 || items[0].Service.Name != summary.Service.Name {
		t.Fatalf("summaries = %#v", items)
	}
}

func TestGetRuntimeServiceDetailsReturnsDeploymentsAndUsages(t *testing.T) {
	fixture := newFixture(t)
	service := testRuntimeService(t, "service-1", "billing-service", fixture.clock.now)
	deployment := testRuntimeDeployment(t, "deployment-1", service, "prod", fixture.clock.now)
	usage := testRuntimeUsage(t, "usage-1", deployment.ID, "user-api", "v1.0.0", domain.RuntimeDriftStatusUpToDate)
	fixture.runtime.details[service.Name.String()] = domain.RuntimeServiceDetails{Service: service, Deployments: []domain.RuntimeDeployment{deployment}, Usages: []domain.RuntimeModuleUsage{usage}}

	details, err := fixture.service.GetRuntimeServiceDetails(context.Background(), "billing-service")
	if err != nil {
		t.Fatalf("get runtime service details: %v", err)
	}
	if details.Service.Name != service.Name || len(details.Deployments) != 1 || len(details.Usages) != 1 {
		t.Fatalf("details = %#v", details)
	}
}

func TestGetEnvironmentInventoryReturnsEnvironmentData(t *testing.T) {
	fixture := newFixture(t)
	environment, _ := domain.NewRuntimeEnvironment("prod")
	fixture.runtime.environmentInventory[environment.String()] = domain.RuntimeEnvironmentInventory{Environment: environment}

	inventory, err := fixture.service.GetEnvironmentInventory(context.Background(), "prod", 20, 0)
	if err != nil {
		t.Fatalf("get environment inventory: %v", err)
	}
	if inventory.Environment != environment {
		t.Fatalf("inventory = %#v", inventory)
	}
}

func TestGetModuleRuntimeUsagesReturnsServiceEnvironmentUsages(t *testing.T) {
	fixture := newFixture(t)
	module := fixture.addModule(t, "user-api", "module-1")
	usage := testModuleRuntimeUsage(t, "billing-service", "prod", "user-api", "v1.0.0")
	fixture.runtime.moduleUsagesByID[module.ID.String()] = []domain.ModuleRuntimeUsage{usage}

	items, err := fixture.service.GetModuleRuntimeUsages(context.Background(), "user-api", 20, 0)
	if err != nil {
		t.Fatalf("get module runtime usages: %v", err)
	}
	if len(items) != 1 || items[0].ServiceName.String() != "billing-service" || items[0].Environment.String() != "prod" {
		t.Fatalf("module usages = %#v", items)
	}
}

func TestGetModuleRuntimeUsagesFallsBackToModuleNameForUnknownModule(t *testing.T) {
	fixture := newFixture(t)
	usage := testModuleRuntimeUsage(t, "billing-service", "prod", "missing-api", "v1.0.0")
	fixture.runtime.moduleUsagesByName["missing-api"] = []domain.ModuleRuntimeUsage{usage}

	items, err := fixture.service.GetModuleRuntimeUsages(context.Background(), "missing-api", 20, 0)
	if err != nil {
		t.Fatalf("get module runtime usages: %v", err)
	}
	if len(items) != 1 || items[0].ModuleName.String() != "missing-api" {
		t.Fatalf("module usages = %#v", items)
	}
}

func TestGetBreakingReportRuntimeImpactReturnsExactBaseVersionMatches(t *testing.T) {
	fixture := newFixture(t)
	report := testBreakingReport(t, "report-1", "version-1")
	fixture.reports.reports[report.ID.String()] = report
	impact := testRuntimeImpact(t, "billing-service", "prod", "user-api", "v1.0.0")
	fixture.runtime.impactByVersion[report.BaseVersionID.String()] = []domain.RuntimeImpact{impact}

	items, err := fixture.service.GetBreakingReportRuntimeImpact(context.Background(), report.ID, 20, 0)
	if err != nil {
		t.Fatalf("get runtime impact: %v", err)
	}
	if len(items) != 1 || items[0].ImpactStatus != domain.RuntimeImpactStatusPotentiallyAffectedByBreakingChange {
		t.Fatalf("impact = %#v", items)
	}
}

func TestGetBreakingReportRuntimeImpactReturnsEmptyWhenNoServicesUseBaseVersion(t *testing.T) {
	fixture := newFixture(t)
	report := testBreakingReport(t, "report-1", "version-1")
	fixture.reports.reports[report.ID.String()] = report

	items, err := fixture.service.GetBreakingReportRuntimeImpact(context.Background(), report.ID, 20, 0)
	if err != nil {
		t.Fatalf("get runtime impact: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("impact = %#v, want empty", items)
	}
}

func validReport(modules []ReportedModuleInput) ReportRuntimeInventoryInput {
	return ReportRuntimeInventoryInput{
		ServiceName:  "billing-service",
		Environment:  "prod",
		GitCommit:    "abc123",
		BuildVersion: "pipeline-1",
		Modules:      modules,
	}
}

type fixture struct {
	service      *Service
	modules      *fakeModuleRepository
	versions     *fakeModuleVersionRepository
	runtime      *fakeRuntimeInventoryRepository
	reports      *fakeBreakingReportRepository
	transactions *fakeTransactions
	outbox       *fakeOutboxWriter
	clock        *fakeClock
	ids          *fakeIDs
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	modules := newFakeModuleRepository()
	versions := newFakeModuleVersionRepository()
	runtime := newFakeRuntimeInventoryRepository()
	reports := newFakeBreakingReportRepository()
	outboxWriter := &fakeOutboxWriter{}
	clock := &fakeClock{now: time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)}
	ids := &fakeIDs{}
	transactions := &fakeTransactions{runtime: runtime, outbox: outboxWriter}
	return &fixture{
		service:      NewService(modules, versions, runtime, reports, transactions, outboxWriter, clock, ids),
		modules:      modules,
		versions:     versions,
		runtime:      runtime,
		reports:      reports,
		transactions: transactions,
		outbox:       outboxWriter,
		clock:        clock,
		ids:          ids,
	}
}

func (f *fixture) addModule(t *testing.T, nameValue string, id string) domain.Module {
	t.Helper()
	name, err := domain.NewModuleName(nameValue)
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	module := domain.Module{ID: domain.NewModuleID(id), Name: name, CreatedAt: f.clock.now, UpdatedAt: f.clock.now}
	f.modules.byName[name.String()] = module
	return module
}

func (f *fixture) addModuleVersion(t *testing.T, nameValue string, versionValue string, moduleID string, versionID string, createdAt time.Time) domain.ModuleVersion {
	t.Helper()
	module, ok := f.modules.byName[nameValue]
	if !ok {
		module = f.addModule(t, nameValue, moduleID)
	}
	version, err := domain.NewVersion(versionValue)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	moduleVersion := domain.ModuleVersion{ID: domain.NewModuleVersionID(versionID), ModuleID: module.ID, Version: version, Status: domain.ModuleVersionStatusPublished, CreatedAt: createdAt}
	f.versions.add(moduleVersion)
	return moduleVersion
}

type fakeModuleRepository struct {
	byName map[string]domain.Module
}

func newFakeModuleRepository() *fakeModuleRepository {
	return &fakeModuleRepository{byName: map[string]domain.Module{}}
}

func (repo *fakeModuleRepository) Create(context.Context, domain.Module) error { return nil }
func (repo *fakeModuleRepository) GetByID(context.Context, domain.ModuleID) (domain.Module, error) {
	return domain.Module{}, domain.ErrNotFound
}
func (repo *fakeModuleRepository) GetByName(_ context.Context, name domain.ModuleName) (domain.Module, error) {
	module, ok := repo.byName[name.String()]
	if !ok {
		return domain.Module{}, domain.ErrNotFound
	}
	return module, nil
}
func (repo *fakeModuleRepository) List(context.Context, int, int) ([]domain.Module, error) {
	return nil, nil
}

type fakeModuleVersionRepository struct {
	byModuleVersion map[string]domain.ModuleVersion
	byModule        map[string][]domain.ModuleVersion
}

func newFakeModuleVersionRepository() *fakeModuleVersionRepository {
	return &fakeModuleVersionRepository{byModuleVersion: map[string]domain.ModuleVersion{}, byModule: map[string][]domain.ModuleVersion{}}
}

func (repo *fakeModuleVersionRepository) add(version domain.ModuleVersion) {
	key := version.ModuleID.String() + "@" + version.Version.String()
	repo.byModuleVersion[key] = version
	repo.byModule[version.ModuleID.String()] = append(repo.byModule[version.ModuleID.String()], version)
}

func (repo *fakeModuleVersionRepository) Create(context.Context, domain.ModuleVersion) error {
	return nil
}
func (repo *fakeModuleVersionRepository) GetByID(context.Context, domain.ModuleVersionID) (domain.ModuleVersion, error) {
	return domain.ModuleVersion{}, domain.ErrNotFound
}
func (repo *fakeModuleVersionRepository) GetByModuleAndVersion(_ context.Context, moduleID domain.ModuleID, version domain.Version) (domain.ModuleVersion, error) {
	item, ok := repo.byModuleVersion[moduleID.String()+"@"+version.String()]
	if !ok {
		return domain.ModuleVersion{}, domain.ErrNotFound
	}
	return item, nil
}
func (repo *fakeModuleVersionRepository) GetLatestByModule(_ context.Context, moduleID domain.ModuleID) (domain.ModuleVersion, error) {
	items := repo.byModule[moduleID.String()]
	if len(items) == 0 {
		return domain.ModuleVersion{}, domain.ErrNotFound
	}
	latest := items[0]
	for _, item := range items[1:] {
		if item.CreatedAt.After(latest.CreatedAt) {
			latest = item
		}
	}
	return latest, nil
}
func (repo *fakeModuleVersionRepository) ListByModule(context.Context, domain.ModuleID, int, int) ([]domain.ModuleVersion, error) {
	return nil, nil
}

type fakeRuntimeInventoryRepository struct {
	servicesByName       map[string]domain.RuntimeService
	deployments          []domain.RuntimeDeployment
	usages               []domain.RuntimeModuleUsage
	summaries            []domain.RuntimeServiceSummary
	details              map[string]domain.RuntimeServiceDetails
	environmentInventory map[string]domain.RuntimeEnvironmentInventory
	moduleUsagesByID     map[string][]domain.ModuleRuntimeUsage
	moduleUsagesByName   map[string][]domain.ModuleRuntimeUsage
	impactByVersion      map[string][]domain.RuntimeImpact
	tx                   *fakeTransactions
}

func newFakeRuntimeInventoryRepository() *fakeRuntimeInventoryRepository {
	return &fakeRuntimeInventoryRepository{
		servicesByName:       map[string]domain.RuntimeService{},
		details:              map[string]domain.RuntimeServiceDetails{},
		environmentInventory: map[string]domain.RuntimeEnvironmentInventory{},
		moduleUsagesByID:     map[string][]domain.ModuleRuntimeUsage{},
		moduleUsagesByName:   map[string][]domain.ModuleRuntimeUsage{},
		impactByVersion:      map[string][]domain.RuntimeImpact{},
	}
}

func (repo *fakeRuntimeInventoryRepository) snapshot() runtimeSnapshot {
	services := map[string]domain.RuntimeService{}
	for key, value := range repo.servicesByName {
		services[key] = value
	}
	return runtimeSnapshot{servicesByName: services, deployments: slices.Clone(repo.deployments), usages: slices.Clone(repo.usages)}
}

func (repo *fakeRuntimeInventoryRepository) restore(snapshot runtimeSnapshot) {
	repo.servicesByName = snapshot.servicesByName
	repo.deployments = snapshot.deployments
	repo.usages = snapshot.usages
}

func (repo *fakeRuntimeInventoryRepository) UpsertRuntimeServiceByName(_ context.Context, service domain.RuntimeService) (domain.RuntimeService, error) {
	if repo.tx != nil && repo.tx.inTransaction {
		repo.tx.inTransactionAtRuntimeWrite = true
	}
	if existing, ok := repo.servicesByName[service.Name.String()]; ok {
		existing.UpdatedAt = service.UpdatedAt
		repo.servicesByName[service.Name.String()] = existing
		return existing, nil
	}
	repo.servicesByName[service.Name.String()] = service
	return service, nil
}
func (repo *fakeRuntimeInventoryRepository) GetRuntimeServiceByName(_ context.Context, serviceName domain.RuntimeServiceName) (domain.RuntimeService, error) {
	service, ok := repo.servicesByName[serviceName.String()]
	if !ok {
		return domain.RuntimeService{}, domain.ErrNotFound
	}
	return service, nil
}
func (repo *fakeRuntimeInventoryRepository) CreateRuntimeDeployment(_ context.Context, deployment domain.RuntimeDeployment) error {
	if repo.tx != nil && repo.tx.inTransaction {
		repo.tx.inTransactionAtRuntimeWrite = true
	}
	repo.deployments = append(repo.deployments, deployment)
	return nil
}
func (repo *fakeRuntimeInventoryRepository) CreateRuntimeModuleUsages(_ context.Context, usages []domain.RuntimeModuleUsage) error {
	if repo.tx != nil && repo.tx.inTransaction {
		repo.tx.inTransactionAtRuntimeWrite = true
	}
	repo.usages = append(repo.usages, usages...)
	return nil
}
func (repo *fakeRuntimeInventoryRepository) ListRuntimeDeploymentsByService(context.Context, domain.RuntimeServiceID, int, int) ([]domain.RuntimeDeployment, error) {
	return nil, nil
}
func (repo *fakeRuntimeInventoryRepository) ListRuntimeModuleUsagesByDeployment(context.Context, domain.RuntimeDeploymentID) ([]domain.RuntimeModuleUsage, error) {
	return nil, nil
}
func (repo *fakeRuntimeInventoryRepository) ListLatestRuntimeUsagesByServiceEnvironment(context.Context, domain.RuntimeServiceName, domain.RuntimeEnvironment) ([]domain.RuntimeModuleUsage, error) {
	return nil, nil
}
func (repo *fakeRuntimeInventoryRepository) ListRuntimeServices(context.Context, int, int) ([]domain.RuntimeServiceSummary, error) {
	return repo.summaries, nil
}
func (repo *fakeRuntimeInventoryRepository) GetRuntimeServiceDetails(_ context.Context, serviceName domain.RuntimeServiceName) (domain.RuntimeServiceDetails, error) {
	return repo.details[serviceName.String()], nil
}
func (repo *fakeRuntimeInventoryRepository) ListRuntimeEnvironmentInventory(_ context.Context, environment domain.RuntimeEnvironment, _ int, _ int) (domain.RuntimeEnvironmentInventory, error) {
	return repo.environmentInventory[environment.String()], nil
}
func (repo *fakeRuntimeInventoryRepository) ListModuleRuntimeUsages(_ context.Context, moduleID domain.ModuleID, _ int, _ int) ([]domain.ModuleRuntimeUsage, error) {
	return repo.moduleUsagesByID[moduleID.String()], nil
}
func (repo *fakeRuntimeInventoryRepository) ListModuleRuntimeUsagesByModuleName(_ context.Context, moduleName domain.ModuleName, _ int, _ int) ([]domain.ModuleRuntimeUsage, error) {
	return repo.moduleUsagesByName[moduleName.String()], nil
}
func (repo *fakeRuntimeInventoryRepository) ListRuntimeModuleUsagesByDriftStatus(context.Context, domain.RuntimeDriftStatus, int, int) ([]domain.RuntimeModuleUsage, error) {
	return nil, nil
}
func (repo *fakeRuntimeInventoryRepository) ListRuntimeImpactByModuleVersion(_ context.Context, _ domain.BreakingReportID, moduleVersionID domain.ModuleVersionID, _ int, _ int) ([]domain.RuntimeImpact, error) {
	return repo.impactByVersion[moduleVersionID.String()], nil
}

type runtimeSnapshot struct {
	servicesByName map[string]domain.RuntimeService
	deployments    []domain.RuntimeDeployment
	usages         []domain.RuntimeModuleUsage
}

type fakeBreakingReportRepository struct {
	reports map[string]domain.BreakingReport
}

func newFakeBreakingReportRepository() *fakeBreakingReportRepository {
	return &fakeBreakingReportRepository{reports: map[string]domain.BreakingReport{}}
}

func (repo *fakeBreakingReportRepository) Create(context.Context, domain.BreakingReport, []domain.BreakingChange) error {
	return nil
}
func (repo *fakeBreakingReportRepository) GetByID(_ context.Context, id domain.BreakingReportID) (domain.BreakingReport, []domain.BreakingChange, error) {
	report, ok := repo.reports[id.String()]
	if !ok {
		return domain.BreakingReport{}, nil, domain.ErrNotFound
	}
	return report, nil, nil
}
func (repo *fakeBreakingReportRepository) ListByModule(context.Context, domain.ModuleID, int, int) ([]domain.BreakingReport, error) {
	return nil, nil
}
func (repo *fakeBreakingReportRepository) CountChangesByReport(context.Context, domain.BreakingReportID) (int, error) {
	return 0, nil
}
func (repo *fakeBreakingReportRepository) ListChangesByReport(context.Context, domain.BreakingReportID, int, int) ([]domain.BreakingChange, error) {
	return nil, nil
}

type fakeTransactions struct {
	runtime                     *fakeRuntimeInventoryRepository
	outbox                      *fakeOutboxWriter
	inTransaction               bool
	inTransactionAtRuntimeWrite bool
	inTransactionAtOutboxWrite  bool
}

func (tx *fakeTransactions) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	tx.runtime.tx = tx
	tx.outbox.tx = tx
	runtimeSnapshot := tx.runtime.snapshot()
	outboxSnapshot := slices.Clone(tx.outbox.records)
	tx.inTransaction = true
	err := fn(ctx)
	tx.inTransaction = false
	if err != nil {
		tx.runtime.restore(runtimeSnapshot)
		tx.outbox.records = outboxSnapshot
		return err
	}
	return nil
}

type fakeOutboxWriter struct {
	records   []outbox.Record
	createErr error
	tx        *fakeTransactions
}

func (writer *fakeOutboxWriter) Create(_ context.Context, record outbox.Record) error {
	if writer.createErr != nil {
		return writer.createErr
	}
	if writer.tx != nil && writer.tx.inTransaction {
		writer.tx.inTransactionAtOutboxWrite = true
	}
	writer.records = append(writer.records, record)
	return nil
}

type fakeClock struct{ now time.Time }

func (clock *fakeClock) Now() time.Time { return clock.now }

type fakeIDs struct {
	serviceID    int
	deploymentID int
	usageID      int
}

func (ids *fakeIDs) NewRuntimeServiceID() (domain.RuntimeServiceID, error) {
	ids.serviceID++
	return domain.NewRuntimeServiceID("service-" + itoa(ids.serviceID)), nil
}
func (ids *fakeIDs) NewRuntimeDeploymentID() (domain.RuntimeDeploymentID, error) {
	ids.deploymentID++
	return domain.NewRuntimeDeploymentID("deployment-" + itoa(ids.deploymentID)), nil
}
func (ids *fakeIDs) NewRuntimeModuleUsageID() (domain.RuntimeModuleUsageID, error) {
	ids.usageID++
	return domain.NewRuntimeModuleUsageID("usage-" + itoa(ids.usageID)), nil
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

func testRuntimeService(t *testing.T, id string, nameValue string, now time.Time) domain.RuntimeService {
	t.Helper()
	name, err := domain.NewRuntimeServiceName(nameValue)
	if err != nil {
		t.Fatalf("service name: %v", err)
	}
	return domain.RuntimeService{ID: domain.NewRuntimeServiceID(id), Name: name, CreatedAt: now, UpdatedAt: now}
}

func testRuntimeDeployment(t *testing.T, id string, service domain.RuntimeService, environmentValue string, now time.Time) domain.RuntimeDeployment {
	t.Helper()
	environment, err := domain.NewRuntimeEnvironment(environmentValue)
	if err != nil {
		t.Fatalf("environment: %v", err)
	}
	return domain.RuntimeDeployment{ID: domain.NewRuntimeDeploymentID(id), ServiceID: service.ID, ServiceName: service.Name, Environment: environment, GitCommit: "abc123", BuildVersion: "pipeline-1", ReportedAt: now, CreatedAt: now}
}

func testRuntimeUsage(t *testing.T, id string, deploymentID domain.RuntimeDeploymentID, moduleNameValue string, versionValue string, status domain.RuntimeDriftStatus) domain.RuntimeModuleUsage {
	t.Helper()
	moduleName, err := domain.NewModuleName(moduleNameValue)
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	version, err := domain.NewVersion(versionValue)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	return domain.RuntimeModuleUsage{ID: domain.NewRuntimeModuleUsageID(id), DeploymentID: deploymentID, ModuleName: moduleName, Version: version, DriftStatus: status}
}

func testRuntimeServiceSummary(t *testing.T, serviceNameValue string) domain.RuntimeServiceSummary {
	t.Helper()
	serviceName, _ := domain.NewRuntimeServiceName(serviceNameValue)
	environment, _ := domain.NewRuntimeEnvironment("prod")
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	return domain.RuntimeServiceSummary{Service: domain.RuntimeService{ID: domain.NewRuntimeServiceID("service-1"), Name: serviceName}, Environments: []domain.RuntimeEnvironment{environment}, EnvironmentCount: 1, DeploymentCount: 1, UpToDateCount: 1, LatestReportedAt: &now}
}

func testModuleRuntimeUsage(t *testing.T, serviceNameValue string, environmentValue string, moduleNameValue string, versionValue string) domain.ModuleRuntimeUsage {
	t.Helper()
	serviceName, _ := domain.NewRuntimeServiceName(serviceNameValue)
	environment, _ := domain.NewRuntimeEnvironment(environmentValue)
	moduleName, _ := domain.NewModuleName(moduleNameValue)
	version, _ := domain.NewVersion(versionValue)
	return domain.ModuleRuntimeUsage{ServiceName: serviceName, Environment: environment, ModuleName: moduleName, Version: version, DriftStatus: domain.RuntimeDriftStatusUpToDate}
}

func testRuntimeImpact(t *testing.T, serviceNameValue string, environmentValue string, moduleNameValue string, versionValue string) domain.RuntimeImpact {
	t.Helper()
	serviceName, _ := domain.NewRuntimeServiceName(serviceNameValue)
	environment, _ := domain.NewRuntimeEnvironment(environmentValue)
	moduleName, _ := domain.NewModuleName(moduleNameValue)
	version, _ := domain.NewVersion(versionValue)
	return domain.RuntimeImpact{ServiceName: serviceName, Environment: environment, UsedModule: moduleName, UsedVersion: version, ImpactStatus: domain.RuntimeImpactStatusPotentiallyAffectedByBreakingChange}
}

func testBreakingReport(t *testing.T, reportID string, baseVersionID string) domain.BreakingReport {
	t.Helper()
	moduleName, _ := domain.NewModuleName("user-api")
	version, _ := domain.NewVersion("v1.0.0")
	return domain.BreakingReport{ID: domain.NewBreakingReportID(reportID), ModuleID: domain.NewModuleID("module-1"), ModuleName: moduleName, BaseVersionID: domain.NewModuleVersionID(baseVersionID), BaseVersion: version}
}
