package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/outbox"
	"github.com/alryzden/ProtoRadar/internal/usecase/runtimeinventory"
)

func TestOutboxLifecycleStatusesAccepted(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	for _, status := range []string{"pending", "processing", "published", "failed", "dead"} {
		if _, err := insertOutboxRecordWithStatus(ctx, db, status, "status-"+status); err != nil {
			t.Fatalf("insert status %q: %v", status, err)
		}
	}
}

func TestOutboxLifecycleInvalidStatusRejected(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_, err := insertOutboxRecordWithStatus(ctx, db, "unknown", "invalid-status")
	if err == nil {
		t.Fatalf("insert invalid status succeeded")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("error = %T %[1]v, want pg error", err)
	}
	if pgErr.ConstraintName != "outbox_records_status_check" {
		t.Fatalf("constraint = %q, want outbox_records_status_check", pgErr.ConstraintName)
	}
}

func TestOutboxLifecycleDefaults(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_, err := db.executor(ctx).Exec(ctx, `
		INSERT INTO outbox_records (event_type, aggregate_type, aggregate_id, dedup_key, payload)
		VALUES ($1, $2, $3, $4, $5)
	`, "protoradar.test.created", "test", "aggregate-defaults", "outbox-defaults", []byte(`{"ok":true}`))
	if err != nil {
		t.Fatalf("insert with defaults: %v", err)
	}

	var status string
	var attempts int
	var availableAt time.Time
	var createdAt time.Time
	var lastError string
	var updatedAt time.Time
	var claimedAt sql.NullTime
	var claimExpiresAt sql.NullTime
	var publishedAt sql.NullTime
	if err := db.executor(ctx).QueryRow(ctx, `
		SELECT status, attempts, available_at, created_at, last_error, updated_at,
		       claimed_at, claim_expires_at, published_at
		FROM outbox_records
		WHERE dedup_key = $1
	`, "outbox-defaults").Scan(
		&status,
		&attempts,
		&availableAt,
		&createdAt,
		&lastError,
		&updatedAt,
		&claimedAt,
		&claimExpiresAt,
		&publishedAt,
	); err != nil {
		t.Fatalf("read defaults: %v", err)
	}

	if status != "pending" {
		t.Fatalf("status = %q, want pending", status)
	}
	if attempts != 0 {
		t.Fatalf("attempts = %d, want 0", attempts)
	}
	if availableAt.IsZero() || createdAt.IsZero() || updatedAt.IsZero() {
		t.Fatalf("timestamps should default: available=%s created=%s updated=%s", availableAt, createdAt, updatedAt)
	}
	if lastError != "" {
		t.Fatalf("last_error = %q, want empty", lastError)
	}
	if claimedAt.Valid || claimExpiresAt.Valid || publishedAt.Valid {
		t.Fatalf("nullable lifecycle timestamps should default null: claimed=%v expires=%v published=%v", claimedAt, claimExpiresAt, publishedAt)
	}
}

func TestOutboxLifecycleDedupKeyUniquenessStillApplies(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	writer := NewOutboxWriter(db)
	record := outbox.Record{
		AggregateType: "test",
		AggregateID:   "aggregate-dedup",
		EventType:     "protoradar.test.created",
		DedupKey:      "outbox-dedup-key",
		Payload:       []byte(`{"ok":true}`),
		OccurredAt:    time.Now().UTC(),
	}

	if err := writer.Create(ctx, record); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if err := writer.Create(ctx, record); !errors.Is(err, domain.ErrDuplicate) {
		t.Fatalf("second create error = %v, want ErrDuplicate", err)
	}
}

func TestOutboxLifecycleIndexesExistAndDoNotBreakInserts(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	for _, indexName := range []string{
		"outbox_records_available_idx",
		"outbox_records_claim_expiry_idx",
		"outbox_records_published_at_idx",
	} {
		var exists bool
		if err := db.executor(ctx).QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, indexName).Scan(&exists); err != nil {
			t.Fatalf("check index %s: %v", indexName, err)
		}
		if !exists {
			t.Fatalf("index %s does not exist", indexName)
		}
	}

	if _, err := insertOutboxRecordWithStatus(ctx, db, "pending", "indexed-insert"); err != nil {
		t.Fatalf("insert with lifecycle indexes: %v", err)
	}
}

func TestOutboxClaimPendingClaimsPendingRecords(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewOutboxWriter(db)
	now := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	id, err := insertOutboxLifecycleRecord(ctx, db, outboxLifecycleRecordInput{Status: "pending", DedupKey: "claim-pending", AvailableAt: now.Add(-time.Minute)})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	records, err := repo.ClaimPending(ctx, 10, time.Minute, now)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(records) != 1 || records[0].ID != id {
		t.Fatalf("records = %#v, want id %s", records, id)
	}
	if records[0].Status != "processing" || records[0].Attempts != 1 || records[0].ClaimedAt == nil || records[0].ClaimExpiresAt == nil {
		t.Fatalf("claimed record = %#v", records[0])
	}
	if !records[0].ClaimExpiresAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("claim_expires_at = %s", records[0].ClaimExpiresAt)
	}
}

func TestOutboxClaimPendingRespectsBatchSize(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewOutboxWriter(db)
	now := time.Date(2026, 6, 6, 10, 5, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if _, err := insertOutboxLifecycleRecord(ctx, db, outboxLifecycleRecordInput{Status: "pending", DedupKey: "batch-" + string(rune('a'+i)), AvailableAt: now.Add(-time.Minute)}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	records, err := repo.ClaimPending(ctx, 2, time.Minute, now)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("claimed = %d, want 2", len(records))
	}
}

func TestOutboxClaimPendingSkipsFutureAttempts(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewOutboxWriter(db)
	now := time.Date(2026, 6, 6, 10, 10, 0, 0, time.UTC)
	pastID, err := insertOutboxLifecycleRecord(ctx, db, outboxLifecycleRecordInput{Status: "failed", DedupKey: "past-retry", AvailableAt: now.Add(-time.Second)})
	if err != nil {
		t.Fatalf("insert past: %v", err)
	}
	if _, err := insertOutboxLifecycleRecord(ctx, db, outboxLifecycleRecordInput{Status: "failed", DedupKey: "future-retry", AvailableAt: now.Add(time.Hour)}); err != nil {
		t.Fatalf("insert future: %v", err)
	}

	records, err := repo.ClaimPending(ctx, 10, time.Minute, now)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(records) != 1 || records[0].ID != pastID {
		t.Fatalf("records = %#v, want only %s", records, pastID)
	}
}

func TestOutboxClaimPendingSkipsPublishedDeadAndNonExpiredProcessing(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewOutboxWriter(db)
	now := time.Date(2026, 6, 6, 10, 15, 0, 0, time.UTC)
	if _, err := insertOutboxLifecycleRecord(ctx, db, outboxLifecycleRecordInput{Status: "published", DedupKey: "skip-published", AvailableAt: now.Add(-time.Hour)}); err != nil {
		t.Fatalf("insert published: %v", err)
	}
	if _, err := insertOutboxLifecycleRecord(ctx, db, outboxLifecycleRecordInput{Status: "dead", DedupKey: "skip-dead", AvailableAt: now.Add(-time.Hour)}); err != nil {
		t.Fatalf("insert dead: %v", err)
	}
	futureClaim := now.Add(time.Minute)
	if _, err := insertOutboxLifecycleRecord(ctx, db, outboxLifecycleRecordInput{Status: "processing", DedupKey: "skip-processing", AvailableAt: now.Add(-time.Hour), ClaimExpiresAt: &futureClaim}); err != nil {
		t.Fatalf("insert processing: %v", err)
	}

	records, err := repo.ClaimPending(ctx, 10, time.Minute, now)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("records = %#v, want none", records)
	}
}

func TestOutboxClaimPendingReclaimsExpiredProcessing(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewOutboxWriter(db)
	now := time.Date(2026, 6, 6, 10, 20, 0, 0, time.UTC)
	expired := now.Add(-time.Second)
	id, err := insertOutboxLifecycleRecord(ctx, db, outboxLifecycleRecordInput{Status: "processing", DedupKey: "expired-processing", AvailableAt: now.Add(-time.Hour), Attempts: 2, ClaimExpiresAt: &expired})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	records, err := repo.ClaimPending(ctx, 10, 2*time.Minute, now)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(records) != 1 || records[0].ID != id || records[0].Attempts != 3 {
		t.Fatalf("records = %#v, want reclaimed attempts 3", records)
	}
}

func TestOutboxConcurrentClaimPendingDoesNotClaimSameRecordTwice(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewOutboxWriter(db)
	now := time.Date(2026, 6, 6, 10, 25, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		if _, err := insertOutboxLifecycleRecord(ctx, db, outboxLifecycleRecordInput{Status: "pending", DedupKey: "concurrent-" + string(rune('a'+i)), AvailableAt: now.Add(-time.Minute)}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	var wg sync.WaitGroup
	results := make(chan []outbox.Record, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			records, err := repo.ClaimPending(ctx, 10, time.Minute, now)
			if err != nil {
				errs <- err
				return
			}
			results <- records
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
	}

	seen := map[string]struct{}{}
	for records := range results {
		for _, record := range records {
			if _, ok := seen[record.ID]; ok {
				t.Fatalf("record claimed twice: %s", record.ID)
			}
			seen[record.ID] = struct{}{}
		}
	}
	if len(seen) != 10 {
		t.Fatalf("claimed unique records = %d, want 10", len(seen))
	}
}

func TestOutboxMarkPublishedSetsStatusAndPreventsReclaim(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewOutboxWriter(db)
	now := time.Date(2026, 6, 6, 10, 30, 0, 0, time.UTC)
	if _, err := insertOutboxLifecycleRecord(ctx, db, outboxLifecycleRecordInput{Status: "pending", DedupKey: "mark-published", AvailableAt: now.Add(-time.Minute)}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	records, err := repo.ClaimPending(ctx, 1, time.Minute, now)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	publishedAt := now.Add(time.Second)
	if err := repo.MarkPublished(ctx, records[0].ID, publishedAt); err != nil {
		t.Fatalf("mark published: %v", err)
	}
	if err := repo.MarkPublished(ctx, records[0].ID, publishedAt.Add(time.Hour)); err != nil {
		t.Fatalf("mark published idempotent: %v", err)
	}

	state := readOutboxState(t, ctx, db, records[0].ID)
	if state.Status != "published" || !state.PublishedAt.Valid || !state.PublishedAt.Time.Equal(publishedAt) || state.ClaimedAt.Valid || state.ClaimExpiresAt.Valid {
		t.Fatalf("state = %#v", state)
	}
	reclaimed, err := repo.ClaimPending(ctx, 10, time.Minute, now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if len(reclaimed) != 0 {
		t.Fatalf("published record reclaimed: %#v", reclaimed)
	}
}

func TestOutboxMarkPublishedUnknownRecordReturnsNotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewOutboxWriter(db)
	err := repo.MarkPublished(context.Background(), "00000000-0000-0000-0000-000000009999", time.Now().UTC())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestOutboxMarkFailedSchedulesRetry(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewOutboxWriter(db)
	now := time.Date(2026, 6, 6, 10, 35, 0, 0, time.UTC)
	if _, err := insertOutboxLifecycleRecord(ctx, db, outboxLifecycleRecordInput{Status: "pending", DedupKey: "mark-failed", AvailableAt: now.Add(-time.Minute)}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	records, err := repo.ClaimPending(ctx, 1, time.Minute, now)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	nextAttemptAt := now.Add(5 * time.Minute)
	if err := repo.MarkFailed(ctx, records[0].ID, "temporary failure", nextAttemptAt, now.Add(time.Second), 3); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	state := readOutboxState(t, ctx, db, records[0].ID)
	if state.Status != "failed" || state.LastError != "temporary failure" || !state.AvailableAt.Equal(nextAttemptAt) || state.ClaimedAt.Valid || state.ClaimExpiresAt.Valid {
		t.Fatalf("state = %#v", state)
	}
}

func TestOutboxMarkFailedMarksDeadAtMaxAttempts(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewOutboxWriter(db)
	now := time.Date(2026, 6, 6, 10, 40, 0, 0, time.UTC)
	id, err := insertOutboxLifecycleRecord(ctx, db, outboxLifecycleRecordInput{Status: "processing", DedupKey: "mark-dead", AvailableAt: now.Add(-time.Minute), Attempts: 3})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := repo.MarkFailed(ctx, id, "permanent failure", now.Add(5*time.Minute), now, 3); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	state := readOutboxState(t, ctx, db, id)
	if state.Status != "dead" || state.LastError != "permanent failure" {
		t.Fatalf("state = %#v", state)
	}
}

func TestOutboxMarkFailedTruncatesLongErrors(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewOutboxWriter(db)
	now := time.Date(2026, 6, 6, 10, 45, 0, 0, time.UTC)
	id, err := insertOutboxLifecycleRecord(ctx, db, outboxLifecycleRecordInput{Status: "processing", DedupKey: "long-error", AvailableAt: now.Add(-time.Minute), Attempts: 1})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	longError := strings.Repeat("x", maxOutboxLastErrorBytes+100)
	if err := repo.MarkFailed(ctx, id, longError, now.Add(time.Minute), now, 3); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	state := readOutboxState(t, ctx, db, id)
	if len(state.LastError) != maxOutboxLastErrorBytes {
		t.Fatalf("last_error length = %d, want %d", len(state.LastError), maxOutboxLastErrorBytes)
	}
}

func TestOutboxStatsReturnsCounts(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewOutboxWriter(db)
	now := time.Date(2026, 6, 6, 10, 50, 0, 0, time.UTC)
	for _, input := range []outboxLifecycleRecordInput{
		{Status: "pending", DedupKey: "stats-pending", AvailableAt: now},
		{Status: "processing", DedupKey: "stats-processing", AvailableAt: now},
		{Status: "published", DedupKey: "stats-published", AvailableAt: now},
		{Status: "failed", DedupKey: "stats-failed", AvailableAt: now},
		{Status: "dead", DedupKey: "stats-dead", AvailableAt: now},
	} {
		if _, err := insertOutboxLifecycleRecord(ctx, db, input); err != nil {
			t.Fatalf("insert %s: %v", input.Status, err)
		}
	}

	stats, err := repo.Stats(ctx)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.Pending != 1 || stats.Processing != 1 || stats.Published != 1 || stats.Failed != 1 || stats.Dead != 1 {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestOutboxLifecycleE2ERuntimeInventoryPublish(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 6, 10, 55, 0, 0, time.UTC)
	module, _ := createGraphModuleVersion(t, ctx, db,
		"00000000-0000-0000-0000-000000092001",
		"user-api",
		"00000000-0000-0000-0000-000000093001",
		"v1.0.0",
		now,
	)

	runtimeRepo := NewRuntimeInventoryRepository(db)
	outboxRepo := NewOutboxWriter(db)
	svc := runtimeinventory.NewService(
		NewModuleRepository(db),
		NewModuleVersionRepository(db),
		runtimeRepo,
		NewBreakingReportRepository(db),
		db,
		outboxRepo,
		fixedOutboxClock{now: now},
		fixedRuntimeInventoryIDs{},
	)

	output, err := svc.ReportRuntimeInventory(ctx, runtimeinventory.ReportRuntimeInventoryInput{
		ServiceName:  "billing-service",
		Environment:  "prod",
		GitCommit:    "abc123",
		BuildVersion: "pipeline-1",
		Modules:      []runtimeinventory.ReportedModuleInput{{Module: module.Name.String(), Version: "v1.0.0"}},
	})
	if err != nil {
		t.Fatalf("report runtime inventory: %v", err)
	}

	var recordID string
	var status string
	var dedupKey string
	if err := db.executor(ctx).QueryRow(ctx, `
		SELECT id, status, dedup_key
		FROM outbox_records
		WHERE aggregate_id = $1
	`, output.DeploymentID).Scan(&recordID, &status, &dedupKey); err != nil {
		t.Fatalf("read pending outbox record: %v", err)
	}
	if status != "pending" || dedupKey != "runtime-deployment:"+output.DeploymentID+":reported" {
		t.Fatalf("outbox status/dedup = %s/%s", status, dedupKey)
	}

	dispatcher := &recordingOutboxDispatcher{}
	publisher, err := outbox.NewPublisher(outboxRepo, dispatcher, outbox.PublisherOptions{
		Enabled:             true,
		BatchSize:           10,
		LeaseDuration:       time.Minute,
		MaxAttempts:         3,
		InitialRetryBackoff: time.Minute,
		MaxRetryBackoff:     time.Hour,
		Clock:               fixedOutboxClock{now: now.Add(time.Second)},
	})
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	if err := publisher.PublishOnce(ctx); err != nil {
		t.Fatalf("publish once: %v", err)
	}
	if len(dispatcher.records) != 1 || dispatcher.records[0].ID != recordID {
		t.Fatalf("dispatched = %#v, want id %s", dispatcher.records, recordID)
	}
	state := readOutboxState(t, ctx, db, recordID)
	if state.Status != "published" || !state.PublishedAt.Valid {
		t.Fatalf("state = %#v", state)
	}

	serviceName, err := domain.NewRuntimeServiceName("billing-service")
	if err != nil {
		t.Fatalf("service name: %v", err)
	}
	storedService, err := runtimeRepo.GetRuntimeServiceByName(ctx, serviceName)
	if err != nil {
		t.Fatalf("get runtime service: %v", err)
	}
	deployments, err := runtimeRepo.ListRuntimeDeploymentsByService(ctx, storedService.ID, 10, 0)
	if err != nil {
		t.Fatalf("list runtime deployments: %v", err)
	}
	if len(deployments) != 1 || deployments[0].ID.String() != output.DeploymentID {
		t.Fatalf("deployments = %#v, want deployment %s", deployments, output.DeploymentID)
	}
}

func TestOutboxPublisherIntegrationPublishesClaimedRecord(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewOutboxWriter(db)
	now := time.Date(2026, 6, 6, 11, 0, 0, 0, time.UTC)
	id, err := insertOutboxLifecycleRecord(ctx, db, outboxLifecycleRecordInput{Status: "pending", DedupKey: "publisher-success", AvailableAt: now.Add(-time.Minute)})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	dispatcher := &recordingOutboxDispatcher{}
	publisher, err := outbox.NewPublisher(repo, dispatcher, outbox.PublisherOptions{
		Enabled:             true,
		BatchSize:           10,
		LeaseDuration:       time.Minute,
		MaxAttempts:         3,
		InitialRetryBackoff: time.Minute,
		MaxRetryBackoff:     time.Hour,
		Clock:               fixedOutboxClock{now: now},
	})
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}

	if err := publisher.PublishOnce(ctx); err != nil {
		t.Fatalf("publish once: %v", err)
	}

	if len(dispatcher.records) != 1 || dispatcher.records[0].ID != id {
		t.Fatalf("dispatched = %#v, want id %s", dispatcher.records, id)
	}
	state := readOutboxState(t, ctx, db, id)
	if state.Status != "published" || !state.PublishedAt.Valid || state.ClaimedAt.Valid || state.ClaimExpiresAt.Valid {
		t.Fatalf("state = %#v", state)
	}
	if state.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", state.Attempts)
	}
	var aggregateID string
	if err := db.executor(ctx).QueryRow(ctx, `SELECT aggregate_id FROM outbox_records WHERE id = $1`, id).Scan(&aggregateID); err != nil {
		t.Fatalf("read aggregate id: %v", err)
	}
	if aggregateID != "publisher-success" {
		t.Fatalf("aggregate id = %q, want publisher-success", aggregateID)
	}
}

func TestOutboxPublisherIntegrationRetriesThenMarksDead(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := NewOutboxWriter(db)
	now := time.Date(2026, 6, 6, 11, 10, 0, 0, time.UTC)
	id, err := insertOutboxLifecycleRecord(ctx, db, outboxLifecycleRecordInput{Status: "pending", DedupKey: "publisher-failure", AvailableAt: now.Add(-time.Minute)})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	clock := &mutableOutboxClock{now: now}
	dispatcher := &recordingOutboxDispatcher{err: errors.New("temporary dispatcher failure")}
	publisher, err := outbox.NewPublisher(repo, dispatcher, outbox.PublisherOptions{
		Enabled:             true,
		BatchSize:           10,
		LeaseDuration:       time.Minute,
		MaxAttempts:         2,
		InitialRetryBackoff: time.Minute,
		MaxRetryBackoff:     time.Hour,
		Clock:               clock,
	})
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}

	if err := publisher.PublishOnce(ctx); err != nil {
		t.Fatalf("first publish once: %v", err)
	}
	state := readOutboxState(t, ctx, db, id)
	if state.Status != "failed" || state.Attempts != 1 || state.LastError != "temporary dispatcher failure" {
		t.Fatalf("state after first failure = %#v", state)
	}
	if !state.AvailableAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("available_at = %s, want %s", state.AvailableAt, now.Add(time.Minute))
	}

	clock.now = now.Add(time.Minute)
	if err := publisher.PublishOnce(ctx); err != nil {
		t.Fatalf("second publish once: %v", err)
	}
	state = readOutboxState(t, ctx, db, id)
	if state.Status != "dead" || state.Attempts != 2 || state.LastError != "temporary dispatcher failure" {
		t.Fatalf("state after second failure = %#v", state)
	}
	if len(dispatcher.records) != 2 {
		t.Fatalf("dispatch count = %d, want 2", len(dispatcher.records))
	}
}

type outboxLifecycleRecordInput struct {
	Status         string
	DedupKey       string
	AvailableAt    time.Time
	Attempts       int
	ClaimExpiresAt *time.Time
}

type outboxState struct {
	Status         string
	Attempts       int
	AvailableAt    time.Time
	PublishedAt    sql.NullTime
	ClaimedAt      sql.NullTime
	ClaimExpiresAt sql.NullTime
	LastError      string
}

func insertOutboxRecordWithStatus(ctx context.Context, db *DB, status string, dedupKey string) (string, error) {
	return insertOutboxLifecycleRecord(ctx, db, outboxLifecycleRecordInput{Status: status, DedupKey: dedupKey, AvailableAt: time.Now().UTC()})
}

func insertOutboxLifecycleRecord(ctx context.Context, db *DB, input outboxLifecycleRecordInput) (string, error) {
	availableAt := input.AvailableAt
	if availableAt.IsZero() {
		availableAt = time.Now().UTC()
	}
	var id string
	err := db.executor(ctx).QueryRow(ctx, `
		INSERT INTO outbox_records (
			event_type,
			aggregate_type,
			aggregate_id,
			dedup_key,
			payload,
			status,
			attempts,
			available_at,
			created_at,
			updated_at,
			claim_expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, $8, $9)
		RETURNING id
	`, "protoradar.test.created", "test", input.DedupKey, input.DedupKey, []byte(`{"ok":true}`), input.Status, input.Attempts, availableAt, nullableTime(input.ClaimExpiresAt)).Scan(&id)
	return id, err
}

func readOutboxState(t *testing.T, ctx context.Context, db *DB, id string) outboxState {
	t.Helper()
	var state outboxState
	if err := db.executor(ctx).QueryRow(ctx, `
		SELECT status, attempts, available_at, published_at, claimed_at, claim_expires_at, last_error
		FROM outbox_records
		WHERE id = $1
	`, id).Scan(&state.Status, &state.Attempts, &state.AvailableAt, &state.PublishedAt, &state.ClaimedAt, &state.ClaimExpiresAt, &state.LastError); err != nil {
		t.Fatalf("read outbox state: %v", err)
	}
	return state
}

type recordingOutboxDispatcher struct {
	records []outbox.Record
	err     error
}

func (dispatcher *recordingOutboxDispatcher) Dispatch(_ context.Context, record outbox.Record) error {
	dispatcher.records = append(dispatcher.records, record)
	return dispatcher.err
}

type fixedOutboxClock struct {
	now time.Time
}

func (clock fixedOutboxClock) Now() time.Time {
	return clock.now
}

type mutableOutboxClock struct {
	now time.Time
}

func (clock *mutableOutboxClock) Now() time.Time {
	return clock.now
}

type fixedRuntimeInventoryIDs struct{}

func (fixedRuntimeInventoryIDs) NewRuntimeServiceID() (domain.RuntimeServiceID, error) {
	return domain.NewRuntimeServiceID("00000000-0000-0000-0000-000000094001"), nil
}

func (fixedRuntimeInventoryIDs) NewRuntimeDeploymentID() (domain.RuntimeDeploymentID, error) {
	return domain.NewRuntimeDeploymentID("00000000-0000-0000-0000-000000095001"), nil
}

func (fixedRuntimeInventoryIDs) NewRuntimeModuleUsageID() (domain.RuntimeModuleUsageID, error) {
	return domain.NewRuntimeModuleUsageID("00000000-0000-0000-0000-000000096001"), nil
}
