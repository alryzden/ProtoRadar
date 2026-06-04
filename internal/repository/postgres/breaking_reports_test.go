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

func TestBreakingReportRepositoryCreateReportWithoutChanges(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v2.0.0")
	repo := NewBreakingReportRepository(db)
	report := testBreakingReport(t, moduleVersionID, "00000000-0000-0000-0000-000000001001", domain.BreakingReportStatusPassed, 0, time.Now().UTC())

	if err := repo.Create(ctx, report, nil); err != nil {
		t.Fatalf("create report: %v", err)
	}

	got, changes, err := repo.GetByID(ctx, report.ID)
	if err != nil {
		t.Fatalf("get report: %v", err)
	}
	if got.ID != report.ID || got.ModuleName.String() != "test-module" || got.BaseVersion.String() != "v2.0.0" {
		t.Fatalf("report = %#v", got)
	}
	if len(changes) != 0 {
		t.Fatalf("changes = %d, want 0", len(changes))
	}
}

func TestBreakingReportRepositoryCreateReportWithChanges(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v2.0.1")
	repo := NewBreakingReportRepository(db)
	report := testBreakingReport(t, moduleVersionID, "00000000-0000-0000-0000-000000001002", domain.BreakingReportStatusBreaking, 2, time.Now().UTC())
	changes := []domain.BreakingChange{
		testBreakingChange(report.ID, "00000000-0000-0000-0000-000000002001", "user.v1.User.email", "FIELD_SAME_TYPE", time.Now().UTC()),
		testBreakingChange(report.ID, "00000000-0000-0000-0000-000000002002", "user.v1.UserService.GetUser", "RPC_NO_DELETE", time.Now().UTC().Add(time.Second)),
	}

	if err := repo.Create(ctx, report, changes); err != nil {
		t.Fatalf("create report: %v", err)
	}

	got, gotChanges, err := repo.GetByID(ctx, report.ID)
	if err != nil {
		t.Fatalf("get report: %v", err)
	}
	if got.Status != domain.BreakingReportStatusBreaking || got.ChangeCount != 2 {
		t.Fatalf("report = %#v", got)
	}
	if len(gotChanges) != 2 {
		t.Fatalf("changes = %d, want 2", len(gotChanges))
	}
	if gotChanges[0].RuleID != "FIELD_SAME_TYPE" || gotChanges[1].RuleID != "RPC_NO_DELETE" {
		t.Fatalf("changes ordered by created_at = %#v", gotChanges)
	}

	count, err := repo.CountChangesByReport(ctx, report.ID)
	if err != nil {
		t.Fatalf("count changes: %v", err)
	}
	if count != 2 {
		t.Fatalf("change count = %d, want 2", count)
	}
}

func TestBreakingReportRepositoryGetByIDNotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewBreakingReportRepository(db)

	if _, _, err := repo.GetByID(ctx, domain.NewBreakingReportID("00000000-0000-0000-0000-000000009999")); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestBreakingReportRepositoryListByModuleOrderedByCreatedAtDesc(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v2.0.2")
	repo := NewBreakingReportRepository(db)
	older := testBreakingReport(t, moduleVersionID, "00000000-0000-0000-0000-000000001003", domain.BreakingReportStatusPassed, 0, time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC))
	newer := testBreakingReport(t, moduleVersionID, "00000000-0000-0000-0000-000000001004", domain.BreakingReportStatusBreaking, 1, time.Date(2026, 6, 4, 11, 0, 0, 0, time.UTC))

	if err := repo.Create(ctx, older, nil); err != nil {
		t.Fatalf("create older report: %v", err)
	}
	if err := repo.Create(ctx, newer, nil); err != nil {
		t.Fatalf("create newer report: %v", err)
	}

	reports, err := repo.ListByModule(ctx, domain.NewModuleID("00000000-0000-0000-0000-000000000001"), 10, 0)
	if err != nil {
		t.Fatalf("list reports: %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("reports = %d, want 2", len(reports))
	}
	if reports[0].ID != newer.ID || reports[1].ID != older.ID {
		t.Fatalf("reports order = %#v", reports)
	}
}

func TestBreakingReportRepositoryCascadeDeletesChangesWhenReportDeleted(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v2.0.3")
	repo := NewBreakingReportRepository(db)
	report := testBreakingReport(t, moduleVersionID, "00000000-0000-0000-0000-000000001005", domain.BreakingReportStatusBreaking, 1, time.Now().UTC())
	changes := []domain.BreakingChange{
		testBreakingChange(report.ID, "00000000-0000-0000-0000-000000002003", "user.v1.User.email", "FIELD_SAME_TYPE", time.Now().UTC()),
	}
	if err := repo.Create(ctx, report, changes); err != nil {
		t.Fatalf("create report: %v", err)
	}

	if _, err := db.executor(ctx).Exec(ctx, `DELETE FROM breaking_reports WHERE id = $1`, report.ID.String()); err != nil {
		t.Fatalf("delete report: %v", err)
	}
	assertTableCount(t, ctx, db, "breaking_changes", 0)
}

func TestBreakingReportRepositoryTransactionRollbackPreventsReportAndChanges(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v2.0.4")
	repo := NewBreakingReportRepository(db)
	report := testBreakingReport(t, moduleVersionID, "00000000-0000-0000-0000-000000001006", domain.BreakingReportStatusBreaking, 1, time.Now().UTC())
	rollbackErr := errors.New("force rollback")

	err := db.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := repo.Create(txCtx, report, []domain.BreakingChange{
			testBreakingChange(report.ID, "00000000-0000-0000-0000-000000002004", "user.v1.User.email", "FIELD_SAME_TYPE", time.Now().UTC()),
		}); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("transaction error = %v", err)
	}
	if _, _, err := repo.GetByID(ctx, report.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("report should not persist after rollback, got %v", err)
	}
	assertTableCount(t, ctx, db, "breaking_changes", 0)
}

func TestBreakingReportRepositoryCanShareTransactionWithOutbox(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v2.0.5")
	repo := NewBreakingReportRepository(db)
	outboxWriter := NewOutboxWriter(db)
	report := testBreakingReport(t, moduleVersionID, "00000000-0000-0000-0000-000000001007", domain.BreakingReportStatusPassed, 0, time.Now().UTC())
	payload, err := json.Marshal(map[string]string{"report_id": report.ID.String()})
	if err != nil {
		t.Fatalf("payload json: %v", err)
	}

	err = db.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := repo.Create(txCtx, report, nil); err != nil {
			return err
		}
		return outboxWriter.Create(txCtx, outbox.Record{
			EventType:     "protoradar.breaking_report.created",
			AggregateType: "module",
			AggregateID:   report.ModuleID.String(),
			DedupKey:      "breaking-report:" + report.ID.String() + ":created",
			Payload:       payload,
			OccurredAt:    report.CreatedAt,
		})
	})
	if err != nil {
		t.Fatalf("transaction: %v", err)
	}
	if _, _, err := repo.GetByID(ctx, report.ID); err != nil {
		t.Fatalf("get report: %v", err)
	}
	assertTableCount(t, ctx, db, "outbox_records", 1)
}

func testBreakingReport(t *testing.T, moduleVersionID domain.ModuleVersionID, id string, status domain.BreakingReportStatus, changeCount int, createdAt time.Time) domain.BreakingReport {
	t.Helper()
	moduleName, err := domain.NewModuleName("test-module")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	version, err := domain.NewVersion("v2.0.0")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	return domain.BreakingReport{
		ID:            domain.NewBreakingReportID(id),
		ModuleID:      domain.NewModuleID("00000000-0000-0000-0000-000000000001"),
		ModuleName:    moduleName,
		BaseVersionID: moduleVersionID,
		BaseVersion:   version,
		TargetRef:     "local",
		Status:        status,
		ChangeCount:   changeCount,
		RawOutput:     "raw buf output",
		HumanSummary:  "human summary",
		CreatedAt:     createdAt,
	}
}

func testBreakingChange(reportID domain.BreakingReportID, id string, symbol string, ruleID string, createdAt time.Time) domain.BreakingChange {
	return domain.BreakingChange{
		ID:          domain.NewBreakingChangeID(id),
		ReportID:    reportID,
		Category:    "field",
		FilePath:    "user/v1/user.proto",
		PackageName: "user.v1",
		Symbol:      symbol,
		RuleID:      ruleID,
		Message:     "breaking change detected",
		Severity:    "error",
		CreatedAt:   createdAt,
	}
}
