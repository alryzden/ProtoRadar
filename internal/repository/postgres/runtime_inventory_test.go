package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/outbox"
)

func TestRuntimeInventoryRepositoryUpsertRuntimeServiceCreatesService(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewRuntimeInventoryRepository(db)
	service := testRuntimeService(t, "00000000-0000-0000-0000-000000080001", "billing-service", time.Now().UTC())

	got, err := repo.UpsertRuntimeServiceByName(ctx, service)
	if err != nil {
		t.Fatalf("upsert service: %v", err)
	}
	if got.ID != service.ID || got.Name != service.Name {
		t.Fatalf("service = %#v, want %#v", got, service)
	}

	stored, err := repo.GetRuntimeServiceByName(ctx, service.Name)
	if err != nil {
		t.Fatalf("get service: %v", err)
	}
	if stored.ID != service.ID || stored.Name != service.Name {
		t.Fatalf("stored service = %#v", stored)
	}
}

func TestRuntimeInventoryRepositoryUpsertRuntimeServiceUpdatesUpdatedAt(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewRuntimeInventoryRepository(db)
	createdAt := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	service := testRuntimeService(t, "00000000-0000-0000-0000-000000080002", "billing-service", createdAt)
	if _, err := repo.UpsertRuntimeServiceByName(ctx, service); err != nil {
		t.Fatalf("upsert service: %v", err)
	}

	updated := service
	updated.ID = domain.NewRuntimeServiceID("00000000-0000-0000-0000-000000080003")
	updated.UpdatedAt = updatedAt
	got, err := repo.UpsertRuntimeServiceByName(ctx, updated)
	if err != nil {
		t.Fatalf("upsert existing service: %v", err)
	}
	if got.ID != service.ID {
		t.Fatalf("service ID = %s, want original %s", got.ID, service.ID)
	}
	if !got.CreatedAt.Equal(createdAt) {
		t.Fatalf("created_at = %s, want %s", got.CreatedAt, createdAt)
	}
	if !got.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("updated_at = %s, want %s", got.UpdatedAt, updatedAt)
	}
}

func TestRuntimeInventoryRepositoryCreateDeploymentAndUsages(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewRuntimeInventoryRepository(db)
	service := createRuntimeService(t, ctx, repo, "00000000-0000-0000-0000-000000080004", "billing-service", time.Now().UTC())
	deployment := testRuntimeDeployment(t, "00000000-0000-0000-0000-000000081001", service, "prod", time.Now().UTC())
	if err := repo.CreateRuntimeDeployment(ctx, deployment); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	module, moduleVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000082001", "user-api", "00000000-0000-0000-0000-000000083001", "v1.0.0", time.Now().UTC())
	usage := testRuntimeUsage(t, "00000000-0000-0000-0000-000000084001", deployment.ID, module.Name, moduleVersion.Version, domain.RuntimeDriftStatusUpToDate)
	usage.ModuleID = &module.ID
	usage.ModuleVersionID = &moduleVersion.ID
	if err := repo.CreateRuntimeModuleUsages(ctx, []domain.RuntimeModuleUsage{usage}); err != nil {
		t.Fatalf("create usages: %v", err)
	}

	deployments, err := repo.ListRuntimeDeploymentsByService(ctx, service.ID, 10, 0)
	if err != nil {
		t.Fatalf("list deployments: %v", err)
	}
	if len(deployments) != 1 || deployments[0].ID != deployment.ID || deployments[0].ServiceName != service.Name {
		t.Fatalf("deployments = %#v", deployments)
	}
	usages, err := repo.ListRuntimeModuleUsagesByDeployment(ctx, deployment.ID)
	if err != nil {
		t.Fatalf("list usages: %v", err)
	}
	if len(usages) != 1 {
		t.Fatalf("usages = %d, want 1", len(usages))
	}
	assertRuntimeUsage(t, usages[0], usage)
}

func TestRuntimeInventoryRepositoryAllowsUnknownUsage(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewRuntimeInventoryRepository(db)
	service := createRuntimeService(t, ctx, repo, "00000000-0000-0000-0000-000000080005", "notification-service", time.Now().UTC())
	deployment := createRuntimeDeployment(t, ctx, repo, "00000000-0000-0000-0000-000000081002", service, "prod", time.Now().UTC())
	moduleName, err := domain.NewModuleName("unknown-api")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	version, err := domain.NewVersion("v9.9.9")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	usage := testRuntimeUsage(t, "00000000-0000-0000-0000-000000084002", deployment.ID, moduleName, version, domain.RuntimeDriftStatusUnknownVersion)

	if err := repo.CreateRuntimeModuleUsages(ctx, []domain.RuntimeModuleUsage{usage}); err != nil {
		t.Fatalf("create unknown usage: %v", err)
	}
	usages, err := repo.ListRuntimeModuleUsagesByDeployment(ctx, deployment.ID)
	if err != nil {
		t.Fatalf("list usages: %v", err)
	}
	if len(usages) != 1 || usages[0].ModuleID != nil || usages[0].ModuleVersionID != nil {
		t.Fatalf("unknown usage should keep nullable IDs: %#v", usages)
	}
}

func TestRuntimeInventoryRepositoryListEnvironmentInventory(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewRuntimeInventoryRepository(db)
	service := createRuntimeService(t, ctx, repo, "00000000-0000-0000-0000-000000080006", "billing-service", time.Now().UTC())
	deployment := createRuntimeDeployment(t, ctx, repo, "00000000-0000-0000-0000-000000081003", service, "prod", time.Now().UTC())
	moduleName, _ := domain.NewModuleName("user-api")
	version, _ := domain.NewVersion("v1.0.0")
	usage := testRuntimeUsage(t, "00000000-0000-0000-0000-000000084003", deployment.ID, moduleName, version, domain.RuntimeDriftStatusBehindLatest)
	if err := repo.CreateRuntimeModuleUsages(ctx, []domain.RuntimeModuleUsage{usage}); err != nil {
		t.Fatalf("create usage: %v", err)
	}

	environment, err := domain.NewRuntimeEnvironment("prod")
	if err != nil {
		t.Fatalf("environment: %v", err)
	}
	inventory, err := repo.ListRuntimeEnvironmentInventory(ctx, environment, 10, 0)
	if err != nil {
		t.Fatalf("list environment inventory: %v", err)
	}
	if inventory.Environment != environment || len(inventory.Deployments) != 1 || len(inventory.Usages) != 1 {
		t.Fatalf("inventory = %#v", inventory)
	}
}

func TestRuntimeInventoryRepositoryListModuleRuntimeUsages(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewRuntimeInventoryRepository(db)
	service := createRuntimeService(t, ctx, repo, "00000000-0000-0000-0000-000000080007", "billing-service", time.Now().UTC())
	deployment := createRuntimeDeployment(t, ctx, repo, "00000000-0000-0000-0000-000000081004", service, "prod", time.Now().UTC())
	module, moduleVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000082002", "user-api", "00000000-0000-0000-0000-000000083002", "v1.0.0", time.Now().UTC())
	usage := testRuntimeUsage(t, "00000000-0000-0000-0000-000000084004", deployment.ID, module.Name, moduleVersion.Version, domain.RuntimeDriftStatusUpToDate)
	usage.ModuleID = &module.ID
	usage.ModuleVersionID = &moduleVersion.ID
	if err := repo.CreateRuntimeModuleUsages(ctx, []domain.RuntimeModuleUsage{usage}); err != nil {
		t.Fatalf("create usage: %v", err)
	}

	byID, err := repo.ListModuleRuntimeUsages(ctx, module.ID, 10, 0)
	if err != nil {
		t.Fatalf("list module usages by id: %v", err)
	}
	if len(byID) != 1 || byID[0].ServiceName != service.Name || byID[0].ModuleName != module.Name {
		t.Fatalf("module usages by id = %#v", byID)
	}
	byName, err := repo.ListModuleRuntimeUsagesByModuleName(ctx, module.Name, 10, 0)
	if err != nil {
		t.Fatalf("list module usages by name: %v", err)
	}
	if len(byName) != 1 || byName[0].DeploymentID != deployment.ID {
		t.Fatalf("module usages by name = %#v", byName)
	}
}

func TestRuntimeInventoryRepositoryDriftStatusFiltering(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewRuntimeInventoryRepository(db)
	service := createRuntimeService(t, ctx, repo, "00000000-0000-0000-0000-000000080008", "billing-service", time.Now().UTC())
	deployment := createRuntimeDeployment(t, ctx, repo, "00000000-0000-0000-0000-000000081005", service, "prod", time.Now().UTC())
	userAPI, _ := domain.NewModuleName("user-api")
	accountAPI, _ := domain.NewModuleName("account-api")
	version, _ := domain.NewVersion("v1.0.0")
	if err := repo.CreateRuntimeModuleUsages(ctx, []domain.RuntimeModuleUsage{
		testRuntimeUsage(t, "00000000-0000-0000-0000-000000084005", deployment.ID, userAPI, version, domain.RuntimeDriftStatusBehindLatest),
		testRuntimeUsage(t, "00000000-0000-0000-0000-000000084006", deployment.ID, accountAPI, version, domain.RuntimeDriftStatusUpToDate),
	}); err != nil {
		t.Fatalf("create usages: %v", err)
	}

	items, err := repo.ListRuntimeModuleUsagesByDriftStatus(ctx, domain.RuntimeDriftStatusBehindLatest, 10, 0)
	if err != nil {
		t.Fatalf("list drift usages: %v", err)
	}
	if len(items) != 1 || items[0].ModuleName != userAPI || items[0].DriftStatus != domain.RuntimeDriftStatusBehindLatest {
		t.Fatalf("drift usages = %#v", items)
	}
}

func TestRuntimeInventoryRepositoryServiceSummary(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewRuntimeInventoryRepository(db)
	service := createRuntimeService(t, ctx, repo, "00000000-0000-0000-0000-000000080009", "billing-service", time.Now().UTC())
	older := createRuntimeDeployment(t, ctx, repo, "00000000-0000-0000-0000-000000081006", service, "prod", time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC))
	latestProd := createRuntimeDeployment(t, ctx, repo, "00000000-0000-0000-0000-000000081007", service, "prod", time.Date(2026, 6, 4, 11, 0, 0, 0, time.UTC))
	staging := createRuntimeDeployment(t, ctx, repo, "00000000-0000-0000-0000-000000081008", service, "staging", time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC))
	userAPI, _ := domain.NewModuleName("user-api")
	version, _ := domain.NewVersion("v1.0.0")
	if err := repo.CreateRuntimeModuleUsages(ctx, []domain.RuntimeModuleUsage{
		testRuntimeUsage(t, "00000000-0000-0000-0000-000000084007", older.ID, userAPI, version, domain.RuntimeDriftStatusUnknownVersion),
		testRuntimeUsage(t, "00000000-0000-0000-0000-000000084008", latestProd.ID, userAPI, version, domain.RuntimeDriftStatusUpToDate),
		testRuntimeUsage(t, "00000000-0000-0000-0000-000000084009", staging.ID, userAPI, version, domain.RuntimeDriftStatusBehindLatest),
	}); err != nil {
		t.Fatalf("create usages: %v", err)
	}

	summaries, err := repo.ListRuntimeServices(ctx, 10, 0)
	if err != nil {
		t.Fatalf("list summaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("summaries = %d, want 1", len(summaries))
	}
	summary := summaries[0]
	if summary.Service.Name != service.Name || summary.EnvironmentCount != 2 || summary.DeploymentCount != 3 {
		t.Fatalf("summary = %#v", summary)
	}
	if len(summary.Environments) != 2 || summary.Environments[0].String() != "prod" || summary.Environments[1].String() != "staging" {
		t.Fatalf("environments = %#v", summary.Environments)
	}
	if summary.UpToDateCount != 1 || summary.BehindLatestCount != 1 || summary.UnknownVersionCount != 0 {
		t.Fatalf("drift summary = %#v", summary)
	}
	if summary.LatestReportedAt == nil || !summary.LatestReportedAt.Equal(staging.ReportedAt) {
		t.Fatalf("last_reported_at = %v, want %s", summary.LatestReportedAt, staging.ReportedAt)
	}
}

func TestRuntimeInventoryRepositoryLatestUsagesByServiceEnvironment(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewRuntimeInventoryRepository(db)
	service := createRuntimeService(t, ctx, repo, "00000000-0000-0000-0000-000000080010", "billing-service", time.Now().UTC())
	older := createRuntimeDeployment(t, ctx, repo, "00000000-0000-0000-0000-000000081009", service, "prod", time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC))
	latest := createRuntimeDeployment(t, ctx, repo, "00000000-0000-0000-0000-000000081010", service, "prod", time.Date(2026, 6, 4, 11, 0, 0, 0, time.UTC))
	olderAPI, _ := domain.NewModuleName("older-api")
	latestAPI, _ := domain.NewModuleName("latest-api")
	version, _ := domain.NewVersion("v1.0.0")
	if err := repo.CreateRuntimeModuleUsages(ctx, []domain.RuntimeModuleUsage{
		testRuntimeUsage(t, "00000000-0000-0000-0000-000000084010", older.ID, olderAPI, version, domain.RuntimeDriftStatusBehindLatest),
		testRuntimeUsage(t, "00000000-0000-0000-0000-000000084011", latest.ID, latestAPI, version, domain.RuntimeDriftStatusUpToDate),
	}); err != nil {
		t.Fatalf("create usages: %v", err)
	}

	environment, _ := domain.NewRuntimeEnvironment("prod")
	usages, err := repo.ListLatestRuntimeUsagesByServiceEnvironment(ctx, service.Name, environment)
	if err != nil {
		t.Fatalf("list latest usages: %v", err)
	}
	if len(usages) != 1 || usages[0].DeploymentID != latest.ID || usages[0].ModuleName != latestAPI {
		t.Fatalf("latest usages = %#v", usages)
	}
}

func TestRuntimeInventoryRepositoryRuntimeImpactByExactModuleVersion(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewRuntimeInventoryRepository(db)
	service := createRuntimeService(t, ctx, repo, "00000000-0000-0000-0000-000000080011", "billing-service", time.Now().UTC())
	deployment := createRuntimeDeployment(t, ctx, repo, "00000000-0000-0000-0000-000000081011", service, "prod", time.Now().UTC())
	module, moduleVersion := createGraphModuleVersion(t, ctx, db, "00000000-0000-0000-0000-000000082003", "user-api", "00000000-0000-0000-0000-000000083003", "v1.0.0", time.Now().UTC())
	usage := testRuntimeUsage(t, "00000000-0000-0000-0000-000000084012", deployment.ID, module.Name, moduleVersion.Version, domain.RuntimeDriftStatusUpToDate)
	usage.ModuleID = &module.ID
	usage.ModuleVersionID = &moduleVersion.ID
	if err := repo.CreateRuntimeModuleUsages(ctx, []domain.RuntimeModuleUsage{usage}); err != nil {
		t.Fatalf("create usage: %v", err)
	}

	impact, err := repo.ListRuntimeImpactByModuleVersion(ctx, domain.NewBreakingReportID("00000000-0000-0000-0000-000000085001"), moduleVersion.ID, 10, 0)
	if err != nil {
		t.Fatalf("list runtime impact: %v", err)
	}
	if len(impact) != 1 || impact[0].ServiceName != service.Name || impact[0].UsedModule != module.Name || impact[0].UsedVersion != moduleVersion.Version {
		t.Fatalf("runtime impact = %#v", impact)
	}
	if impact[0].ImpactStatus != domain.RuntimeImpactStatusPotentiallyAffectedByBreakingChange {
		t.Fatalf("impact status = %q", impact[0].ImpactStatus)
	}
	if impact[0].DriftStatus != domain.RuntimeDriftStatusUpToDate || impact[0].DriftReason != domain.RuntimeDriftStatusUpToDate.String() {
		t.Fatalf("impact drift = %#v", impact[0])
	}
}

func TestRuntimeInventoryRepositoryTransactionRollback(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewRuntimeInventoryRepository(db)
	service := testRuntimeService(t, "00000000-0000-0000-0000-000000080012", "billing-service", time.Now().UTC())
	deployment := testRuntimeDeployment(t, "00000000-0000-0000-0000-000000081012", service, "prod", time.Now().UTC())
	moduleName, _ := domain.NewModuleName("user-api")
	version, _ := domain.NewVersion("v1.0.0")
	usage := testRuntimeUsage(t, "00000000-0000-0000-0000-000000084013", deployment.ID, moduleName, version, domain.RuntimeDriftStatusUpToDate)
	rollbackErr := errors.New("force rollback")

	err := db.WithinTransaction(ctx, func(txCtx context.Context) error {
		createdService, err := repo.UpsertRuntimeServiceByName(txCtx, service)
		if err != nil {
			return err
		}
		deployment.ServiceID = createdService.ID
		if err := repo.CreateRuntimeDeployment(txCtx, deployment); err != nil {
			return err
		}
		if err := repo.CreateRuntimeModuleUsages(txCtx, []domain.RuntimeModuleUsage{usage}); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("transaction error = %v", err)
	}
	assertTableCount(t, ctx, db, "runtime_services", 0)
	assertTableCount(t, ctx, db, "runtime_deployments", 0)
	assertTableCount(t, ctx, db, "runtime_module_usages", 0)
}

func TestRuntimeInventoryRepositoryCanShareTransactionWithOutbox(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewRuntimeInventoryRepository(db)
	outboxWriter := NewOutboxWriter(db)
	service := testRuntimeService(t, "00000000-0000-0000-0000-000000080013", "billing-service", time.Now().UTC())
	deployment := testRuntimeDeployment(t, "00000000-0000-0000-0000-000000081013", service, "prod", time.Now().UTC())
	payload, err := json.Marshal(map[string]string{"deployment_id": deployment.ID.String()})
	if err != nil {
		t.Fatalf("payload json: %v", err)
	}

	err = db.WithinTransaction(ctx, func(txCtx context.Context) error {
		createdService, err := repo.UpsertRuntimeServiceByName(txCtx, service)
		if err != nil {
			return err
		}
		deployment.ServiceID = createdService.ID
		if err := repo.CreateRuntimeDeployment(txCtx, deployment); err != nil {
			return err
		}
		return outboxWriter.Create(txCtx, outbox.Record{
			EventType:     "protoradar.runtime_inventory.reported",
			AggregateType: "runtime_deployment",
			AggregateID:   deployment.ID.String(),
			DedupKey:      "runtime-deployment:" + deployment.ID.String() + ":reported",
			Payload:       payload,
			OccurredAt:    deployment.ReportedAt,
		})
	})
	if err != nil {
		t.Fatalf("transaction: %v", err)
	}
	assertTableCount(t, ctx, db, "runtime_services", 1)
	assertTableCount(t, ctx, db, "runtime_deployments", 1)
	assertTableCount(t, ctx, db, "outbox_records", 1)
}

func TestRuntimeInventoryRepositoryCascadeDeletes(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewRuntimeInventoryRepository(db)
	service := createRuntimeService(t, ctx, repo, "00000000-0000-0000-0000-000000080014", "billing-service", time.Now().UTC())
	deployment := createRuntimeDeployment(t, ctx, repo, "00000000-0000-0000-0000-000000081014", service, "prod", time.Now().UTC())
	moduleName, _ := domain.NewModuleName("user-api")
	version, _ := domain.NewVersion("v1.0.0")
	usage := testRuntimeUsage(t, "00000000-0000-0000-0000-000000084014", deployment.ID, moduleName, version, domain.RuntimeDriftStatusUpToDate)
	if err := repo.CreateRuntimeModuleUsages(ctx, []domain.RuntimeModuleUsage{usage}); err != nil {
		t.Fatalf("create usage: %v", err)
	}

	if _, err := db.executor(ctx).Exec(ctx, `DELETE FROM runtime_deployments WHERE id = $1`, deployment.ID.String()); err != nil {
		t.Fatalf("delete deployment: %v", err)
	}
	assertTableCount(t, ctx, db, "runtime_module_usages", 0)

	deployment = createRuntimeDeployment(t, ctx, repo, "00000000-0000-0000-0000-000000081015", service, "prod", time.Now().UTC())
	usage.ID = domain.NewRuntimeModuleUsageID("00000000-0000-0000-0000-000000084015")
	usage.DeploymentID = deployment.ID
	if err := repo.CreateRuntimeModuleUsages(ctx, []domain.RuntimeModuleUsage{usage}); err != nil {
		t.Fatalf("create usage after recreate: %v", err)
	}
	if _, err := db.executor(ctx).Exec(ctx, `DELETE FROM runtime_services WHERE id = $1`, service.ID.String()); err != nil {
		t.Fatalf("delete service: %v", err)
	}
	assertTableCount(t, ctx, db, "runtime_deployments", 0)
	assertTableCount(t, ctx, db, "runtime_module_usages", 0)
}

func testRuntimeService(t *testing.T, id string, nameValue string, now time.Time) domain.RuntimeService {
	t.Helper()
	name, err := domain.NewRuntimeServiceName(nameValue)
	if err != nil {
		t.Fatalf("runtime service name: %v", err)
	}
	return domain.RuntimeService{ID: domain.NewRuntimeServiceID(id), Name: name, CreatedAt: now, UpdatedAt: now}
}

func createRuntimeService(t *testing.T, ctx context.Context, repo *RuntimeInventoryRepository, id string, name string, now time.Time) domain.RuntimeService {
	t.Helper()
	service, err := repo.UpsertRuntimeServiceByName(ctx, testRuntimeService(t, id, name, now))
	if err != nil {
		t.Fatalf("create runtime service: %v", err)
	}
	return service
}

func testRuntimeDeployment(t *testing.T, id string, service domain.RuntimeService, environmentValue string, reportedAt time.Time) domain.RuntimeDeployment {
	t.Helper()
	environment, err := domain.NewRuntimeEnvironment(environmentValue)
	if err != nil {
		t.Fatalf("runtime environment: %v", err)
	}
	return domain.RuntimeDeployment{
		ID:           domain.NewRuntimeDeploymentID(id),
		ServiceID:    service.ID,
		ServiceName:  service.Name,
		Environment:  environment,
		GitCommit:    "abc123",
		BuildVersion: "pipeline-456",
		ReportedAt:   reportedAt,
		CreatedAt:    reportedAt,
	}
}

func createRuntimeDeployment(t *testing.T, ctx context.Context, repo *RuntimeInventoryRepository, id string, service domain.RuntimeService, environment string, reportedAt time.Time) domain.RuntimeDeployment {
	t.Helper()
	deployment := testRuntimeDeployment(t, id, service, environment, reportedAt)
	if err := repo.CreateRuntimeDeployment(ctx, deployment); err != nil {
		t.Fatalf("create runtime deployment: %v", err)
	}
	return deployment
}

func testRuntimeUsage(t *testing.T, id string, deploymentID domain.RuntimeDeploymentID, moduleName domain.ModuleName, version domain.Version, driftStatus domain.RuntimeDriftStatus) domain.RuntimeModuleUsage {
	t.Helper()
	return domain.RuntimeModuleUsage{
		ID:           domain.NewRuntimeModuleUsageID(id),
		DeploymentID: deploymentID,
		ModuleName:   moduleName,
		Version:      version,
		DriftStatus:  driftStatus,
		DriftReason:  driftStatus.String(),
		CreatedAt:    time.Now().UTC(),
	}
}

func assertRuntimeUsage(t *testing.T, got domain.RuntimeModuleUsage, want domain.RuntimeModuleUsage) {
	t.Helper()
	if got.ID != want.ID || got.DeploymentID != want.DeploymentID || got.ModuleName != want.ModuleName || got.Version != want.Version {
		t.Fatalf("usage = %#v, want %#v", got, want)
	}
	if got.DriftStatus != want.DriftStatus || got.DriftReason != want.DriftReason {
		t.Fatalf("usage drift = %#v, want %#v", got, want)
	}
	if want.ModuleID == nil {
		if got.ModuleID != nil {
			t.Fatalf("module id = %v, want nil", got.ModuleID)
		}
	} else if got.ModuleID == nil || *got.ModuleID != *want.ModuleID {
		t.Fatalf("module id = %v, want %v", got.ModuleID, want.ModuleID)
	}
	if want.ModuleVersionID == nil {
		if got.ModuleVersionID != nil {
			t.Fatalf("module version id = %v, want nil", got.ModuleVersionID)
		}
	} else if got.ModuleVersionID == nil || *got.ModuleVersionID != *want.ModuleVersionID {
		t.Fatalf("module version id = %v, want %v", got.ModuleVersionID, want.ModuleVersionID)
	}
}
