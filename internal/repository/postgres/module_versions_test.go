package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

func TestModuleVersionRepositoryCreateDefaultsToNotDeprecated(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	module := createTestModule(t, ctx, db, "00000000-0000-0000-0000-000000090001", "user-api")
	repo := NewModuleVersionRepository(db)
	version := testModuleVersion(t, module, "00000000-0000-0000-0000-000000091001", "v1.0.0", time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC))

	if err := repo.Create(ctx, version); err != nil {
		t.Fatalf("create module version: %v", err)
	}

	got, err := repo.GetByID(ctx, version.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	assertModuleVersion(t, got, version)
	if got.IsDeprecated() || got.DeprecatedAt != nil || got.DeprecatedBy != "" || got.DeprecationReason != "" {
		t.Fatalf("new module version should not be deprecated: %#v", got)
	}
}

func TestModuleVersionRepositoryPersistsDeprecationMetadata(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	module := createTestModule(t, ctx, db, "00000000-0000-0000-0000-000000090002", "billing-api")
	repo := NewModuleVersionRepository(db)
	deprecatedAt := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	version := testModuleVersion(t, module, "00000000-0000-0000-0000-000000091002", "v1.0.0", deprecatedAt.Add(-time.Hour))
	version.DeprecatedAt = &deprecatedAt
	version.DeprecatedBy = "api-token:platform"
	version.DeprecationReason = "superseded by v1.1.0"

	if err := repo.Create(ctx, version); err != nil {
		t.Fatalf("create deprecated module version: %v", err)
	}

	byID, err := repo.GetByID(ctx, version.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	assertModuleVersion(t, byID, version)
	if !byID.IsDeprecated() {
		t.Fatalf("module version should be deprecated: %#v", byID)
	}

	byModuleVersion, err := repo.GetByModuleAndVersion(ctx, module.ID, version.Version)
	if err != nil {
		t.Fatalf("get by module/version: %v", err)
	}
	assertModuleVersion(t, byModuleVersion, version)

	latest, err := repo.GetLatestByModule(ctx, module.ID)
	if err != nil {
		t.Fatalf("get latest: %v", err)
	}
	assertModuleVersion(t, latest, version)

	listed, err := repo.ListByModule(ctx, module.ID, 10, 0)
	if err != nil {
		t.Fatalf("list by module: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("listed versions = %d, want 1", len(listed))
	}
	assertModuleVersion(t, listed[0], version)
}

func TestModuleVersionRepositoryUpdateDeprecation(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	module := createTestModule(t, ctx, db, "00000000-0000-0000-0000-000000090003", "orders-api")
	repo := NewModuleVersionRepository(db)
	version := testModuleVersion(t, module, "00000000-0000-0000-0000-000000091003", "v1.0.0", time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC))
	if err := repo.Create(ctx, version); err != nil {
		t.Fatalf("create module version: %v", err)
	}

	deprecatedAt := time.Date(2026, 6, 6, 13, 0, 0, 0, time.UTC)
	if err := repo.UpdateDeprecation(ctx, version.ID, &deprecatedAt, "api-token:release", "legacy contract"); err != nil {
		t.Fatalf("update deprecation: %v", err)
	}

	got, err := repo.GetByID(ctx, version.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if !got.IsDeprecated() || got.DeprecatedAt == nil || !got.DeprecatedAt.Equal(deprecatedAt) || got.DeprecatedBy != "api-token:release" || got.DeprecationReason != "legacy contract" {
		t.Fatalf("deprecation metadata = %#v", got)
	}

	if err := repo.UpdateDeprecation(ctx, version.ID, nil, "", ""); err != nil {
		t.Fatalf("clear deprecation: %v", err)
	}
	got, err = repo.GetByID(ctx, version.ID)
	if err != nil {
		t.Fatalf("get cleared by id: %v", err)
	}
	if got.IsDeprecated() || got.DeprecatedAt != nil || got.DeprecatedBy != "" || got.DeprecationReason != "" {
		t.Fatalf("deprecation metadata should be cleared: %#v", got)
	}
}

func TestModuleVersionRepositoryUpdateDeprecationNotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewModuleVersionRepository(db)
	deprecatedAt := time.Now().UTC()

	err := repo.UpdateDeprecation(ctx, domain.NewModuleVersionID("00000000-0000-0000-0000-000000099999"), &deprecatedAt, "api-token:release", "legacy contract")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("update deprecation error = %v, want ErrNotFound", err)
	}
}

func testModuleVersion(t *testing.T, module domain.Module, id string, versionValue string, createdAt time.Time) domain.ModuleVersion {
	t.Helper()
	version, err := domain.NewVersion(versionValue)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	return domain.ModuleVersion{
		ID:        domain.NewModuleVersionID(id),
		ModuleID:  module.ID,
		Version:   version,
		Status:    domain.ModuleVersionStatusPublished,
		Digest:    "sha256:" + versionValue,
		CreatedAt: createdAt,
	}
}

func assertModuleVersion(t *testing.T, got domain.ModuleVersion, want domain.ModuleVersion) {
	t.Helper()
	if got.ID != want.ID || got.ModuleID != want.ModuleID || got.Version != want.Version || got.Status != want.Status || got.Digest != want.Digest {
		t.Fatalf("module version = %#v, want %#v", got, want)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("created_at = %s, want %s", got.CreatedAt, want.CreatedAt)
	}
	if want.DeprecatedAt == nil {
		if got.DeprecatedAt != nil {
			t.Fatalf("deprecated_at = %v, want nil", got.DeprecatedAt)
		}
	} else if got.DeprecatedAt == nil || !got.DeprecatedAt.Equal(*want.DeprecatedAt) {
		t.Fatalf("deprecated_at = %v, want %s", got.DeprecatedAt, *want.DeprecatedAt)
	}
	if got.DeprecatedBy != want.DeprecatedBy || got.DeprecationReason != want.DeprecationReason {
		t.Fatalf("deprecation metadata = %q/%q, want %q/%q", got.DeprecatedBy, got.DeprecationReason, want.DeprecatedBy, want.DeprecationReason)
	}
}
