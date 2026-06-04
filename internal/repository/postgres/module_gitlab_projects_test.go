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

func TestModuleGitLabProjectRepositoryUpsertCreatesMapping(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	module := createTestModule(t, ctx, db, "00000000-0000-0000-0000-000000000101", "user-api")
	repo := NewModuleGitLabProjectRepository(db)
	mapping := testModuleGitLabProject(t, module, "00000000-0000-0000-0000-000000001001", "https://gitlab.example.com", 123, "platform/user-api")

	if err := repo.Upsert(ctx, mapping); err != nil {
		t.Fatalf("upsert mapping: %v", err)
	}

	got, err := repo.GetByModuleID(ctx, module.ID)
	if err != nil {
		t.Fatalf("get by module id: %v", err)
	}
	assertModuleGitLabProject(t, got, mapping)
}

func TestModuleGitLabProjectRepositoryUpsertUpdatesExistingModuleMapping(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	module := createTestModule(t, ctx, db, "00000000-0000-0000-0000-000000000102", "billing-api")
	repo := NewModuleGitLabProjectRepository(db)
	created := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	updated := created.Add(time.Hour)
	initial := testModuleGitLabProject(t, module, "00000000-0000-0000-0000-000000001002", "https://gitlab.example.com", 123, "platform/billing-api")
	initial.CreatedAt = created
	initial.UpdatedAt = created
	changed := testModuleGitLabProject(t, module, "00000000-0000-0000-0000-000000001003", "https://gitlab.example.org", 456, "platform/billing-api-v2")
	changed.CreatedAt = created.Add(2 * time.Hour)
	changed.UpdatedAt = updated

	if err := repo.Upsert(ctx, initial); err != nil {
		t.Fatalf("upsert initial mapping: %v", err)
	}
	if err := repo.Upsert(ctx, changed); err != nil {
		t.Fatalf("upsert changed mapping: %v", err)
	}

	got, err := repo.GetByModuleID(ctx, module.ID)
	if err != nil {
		t.Fatalf("get by module id: %v", err)
	}
	if got.ID != initial.ID {
		t.Fatalf("mapping id = %q, want original %q", got.ID, initial.ID)
	}
	if got.GitLabBaseURL != changed.GitLabBaseURL || got.GitLabProjectID != changed.GitLabProjectID || got.GitLabProjectPath != changed.GitLabProjectPath {
		t.Fatalf("mapping not updated: %#v", got)
	}
	if !got.CreatedAt.Equal(created) {
		t.Fatalf("created_at = %s, want %s", got.CreatedAt, created)
	}
	if !got.UpdatedAt.Equal(updated) {
		t.Fatalf("updated_at = %s, want %s", got.UpdatedAt, updated)
	}
	assertTableCount(t, ctx, db, "module_gitlab_projects", 1)
}

func TestModuleGitLabProjectRepositoryGetByModuleID(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	module := createTestModule(t, ctx, db, "00000000-0000-0000-0000-000000000103", "inventory-api")
	repo := NewModuleGitLabProjectRepository(db)
	mapping := testModuleGitLabProject(t, module, "00000000-0000-0000-0000-000000001004", "https://gitlab.example.com", 234, "platform/inventory-api")
	if err := repo.Upsert(ctx, mapping); err != nil {
		t.Fatalf("upsert mapping: %v", err)
	}

	got, err := repo.GetByModuleID(ctx, module.ID)
	if err != nil {
		t.Fatalf("get by module id: %v", err)
	}
	assertModuleGitLabProject(t, got, mapping)
}

func TestModuleGitLabProjectRepositoryGetByGitLabProject(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	module := createTestModule(t, ctx, db, "00000000-0000-0000-0000-000000000104", "orders-api")
	repo := NewModuleGitLabProjectRepository(db)
	mapping := testModuleGitLabProject(t, module, "00000000-0000-0000-0000-000000001005", "https://gitlab.example.com", 345, "platform/orders-api")
	if err := repo.Upsert(ctx, mapping); err != nil {
		t.Fatalf("upsert mapping: %v", err)
	}

	got, err := repo.GetByGitLabProject(ctx, "https://gitlab.example.com", 345)
	if err != nil {
		t.Fatalf("get by gitlab project: %v", err)
	}
	assertModuleGitLabProject(t, got, mapping)
}

func TestModuleGitLabProjectRepositoryGetNotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewModuleGitLabProjectRepository(db)

	if _, err := repo.GetByModuleID(ctx, domain.NewModuleID("00000000-0000-0000-0000-000000009999")); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("get by module id error = %v, want ErrNotFound", err)
	}
	if _, err := repo.GetByGitLabProject(ctx, "https://gitlab.example.com", 999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("get by gitlab project error = %v, want ErrNotFound", err)
	}
}

func TestModuleGitLabProjectRepositoryUniqueModuleMapping(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	module := createTestModule(t, ctx, db, "00000000-0000-0000-0000-000000000105", "profile-api")
	repo := NewModuleGitLabProjectRepository(db)
	first := testModuleGitLabProject(t, module, "00000000-0000-0000-0000-000000001006", "https://gitlab.example.com", 456, "platform/profile-api")
	second := testModuleGitLabProject(t, module, "00000000-0000-0000-0000-000000001007", "https://gitlab.example.com", 457, "platform/profile-api-v2")

	if err := repo.Upsert(ctx, first); err != nil {
		t.Fatalf("upsert first mapping: %v", err)
	}
	if err := repo.Upsert(ctx, second); err != nil {
		t.Fatalf("upsert second mapping: %v", err)
	}

	assertTableCount(t, ctx, db, "module_gitlab_projects", 1)
	got, err := repo.GetByModuleID(ctx, module.ID)
	if err != nil {
		t.Fatalf("get by module id: %v", err)
	}
	if got.GitLabProjectID != 457 || got.GitLabProjectPath != "platform/profile-api-v2" {
		t.Fatalf("mapping = %#v", got)
	}
}

func TestModuleGitLabProjectRepositoryUniqueGitLabProject(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	firstModule := createTestModule(t, ctx, db, "00000000-0000-0000-0000-000000000106", "payments-api")
	secondModule := createTestModule(t, ctx, db, "00000000-0000-0000-0000-000000000107", "ledger-api")
	repo := NewModuleGitLabProjectRepository(db)
	first := testModuleGitLabProject(t, firstModule, "00000000-0000-0000-0000-000000001008", "https://gitlab.example.com", 567, "platform/payments-api")
	second := testModuleGitLabProject(t, secondModule, "00000000-0000-0000-0000-000000001009", "https://gitlab.example.com", 567, "platform/payments-api")

	if err := repo.Upsert(ctx, first); err != nil {
		t.Fatalf("upsert first mapping: %v", err)
	}
	if err := repo.Upsert(ctx, second); !errors.Is(err, domain.ErrDuplicate) {
		t.Fatalf("upsert conflicting mapping error = %v, want ErrDuplicate", err)
	}

	assertTableCount(t, ctx, db, "module_gitlab_projects", 1)
	got, err := repo.GetByGitLabProject(ctx, "https://gitlab.example.com", 567)
	if err != nil {
		t.Fatalf("get by gitlab project: %v", err)
	}
	if got.ModuleID != firstModule.ID {
		t.Fatalf("gitlab project linked to module %q, want %q", got.ModuleID, firstModule.ID)
	}
}

func TestModuleGitLabProjectRepositoryConflictWhenAnotherModuleLinkedToGitLabProject(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	firstModule := createTestModule(t, ctx, db, "00000000-0000-0000-0000-000000000108", "search-api")
	secondModule := createTestModule(t, ctx, db, "00000000-0000-0000-0000-000000000109", "catalog-api")
	repo := NewModuleGitLabProjectRepository(db)
	first := testModuleGitLabProject(t, firstModule, "00000000-0000-0000-0000-000000001010", "https://gitlab.example.com", 678, "platform/search-api")
	second := testModuleGitLabProject(t, secondModule, "00000000-0000-0000-0000-000000001011", "https://gitlab.example.com", 679, "platform/catalog-api")
	conflictingSecond := testModuleGitLabProject(t, secondModule, "00000000-0000-0000-0000-000000001011", "https://gitlab.example.com", 678, "platform/search-api")

	if err := repo.Upsert(ctx, first); err != nil {
		t.Fatalf("upsert first mapping: %v", err)
	}
	if err := repo.Upsert(ctx, second); err != nil {
		t.Fatalf("upsert second mapping: %v", err)
	}
	if err := repo.Upsert(ctx, conflictingSecond); !errors.Is(err, domain.ErrDuplicate) {
		t.Fatalf("upsert conflicting update error = %v, want ErrDuplicate", err)
	}

	got, err := repo.GetByModuleID(ctx, secondModule.ID)
	if err != nil {
		t.Fatalf("get second mapping: %v", err)
	}
	if got.GitLabProjectID != 679 || got.GitLabProjectPath != "platform/catalog-api" {
		t.Fatalf("conflicting update should not replace existing mapping, got %#v", got)
	}
}

func TestModuleGitLabProjectRepositoryTransactionRollbackPreventsMapping(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	module := createTestModule(t, ctx, db, "00000000-0000-0000-0000-000000000110", "notifications-api")
	repo := NewModuleGitLabProjectRepository(db)
	mapping := testModuleGitLabProject(t, module, "00000000-0000-0000-0000-000000001012", "https://gitlab.example.com", 789, "platform/notifications-api")
	rollbackErr := errors.New("force rollback")

	err := db.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := repo.Upsert(txCtx, mapping); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("transaction error = %v", err)
	}
	if _, err := repo.GetByModuleID(ctx, module.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("mapping should not persist after rollback, got %v", err)
	}
}

func TestModuleGitLabProjectRepositoryCanShareTransactionWithOutbox(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	module := createTestModule(t, ctx, db, "00000000-0000-0000-0000-000000000111", "reports-api")
	repo := NewModuleGitLabProjectRepository(db)
	outboxWriter := NewOutboxWriter(db)
	mapping := testModuleGitLabProject(t, module, "00000000-0000-0000-0000-000000001013", "https://gitlab.example.com", 890, "platform/reports-api")
	payload, err := json.Marshal(map[string]string{"module_id": module.ID.String()})
	if err != nil {
		t.Fatalf("payload json: %v", err)
	}

	err = db.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := repo.Upsert(txCtx, mapping); err != nil {
			return err
		}
		return outboxWriter.Create(txCtx, outbox.Record{
			EventType:     "protoradar.module_gitlab_project.linked",
			AggregateType: "module",
			AggregateID:   module.ID.String(),
			DedupKey:      "module:" + module.ID.String() + ":gitlab-project:https://gitlab.example.com:890:linked",
			Payload:       payload,
			OccurredAt:    mapping.UpdatedAt,
		})
	})
	if err != nil {
		t.Fatalf("transaction: %v", err)
	}
	if _, err := repo.GetByModuleID(ctx, module.ID); err != nil {
		t.Fatalf("get mapping: %v", err)
	}
	assertTableCount(t, ctx, db, "module_gitlab_projects", 1)
	assertTableCount(t, ctx, db, "outbox_records", 1)
}

func createTestModule(t *testing.T, ctx context.Context, db *DB, id string, nameValue string) domain.Module {
	t.Helper()
	moduleName, err := domain.NewModuleName(nameValue)
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	module := domain.Module{
		ID:        domain.NewModuleID(id),
		Name:      moduleName,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := NewModuleRepository(db).Create(ctx, module); err != nil {
		t.Fatalf("create module: %v", err)
	}
	return module
}

func testModuleGitLabProject(t *testing.T, module domain.Module, id string, baseURL string, projectID int64, projectPath string) domain.ModuleGitLabProject {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	mapping := domain.ModuleGitLabProject{
		ID:                domain.NewModuleGitLabProjectID(id),
		ModuleID:          module.ID,
		ModuleName:        module.Name,
		GitLabBaseURL:     baseURL,
		GitLabProjectID:   projectID,
		GitLabProjectPath: projectPath,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := mapping.Validate(); err != nil {
		t.Fatalf("mapping validation: %v", err)
	}
	return mapping
}

func assertModuleGitLabProject(t *testing.T, got domain.ModuleGitLabProject, want domain.ModuleGitLabProject) {
	t.Helper()
	if got.ID != want.ID {
		t.Fatalf("id = %q, want %q", got.ID, want.ID)
	}
	if got.ModuleID != want.ModuleID || got.ModuleName != want.ModuleName {
		t.Fatalf("module = %q/%q, want %q/%q", got.ModuleID, got.ModuleName, want.ModuleID, want.ModuleName)
	}
	if got.GitLabBaseURL != want.GitLabBaseURL || got.GitLabProjectID != want.GitLabProjectID || got.GitLabProjectPath != want.GitLabProjectPath {
		t.Fatalf("gitlab mapping = %#v, want %#v", got, want)
	}
}
