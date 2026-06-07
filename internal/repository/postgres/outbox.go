package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/outbox"
)

const (
	outboxStatusPending     = "pending"
	outboxStatusProcessing  = "processing"
	outboxStatusPublished   = "published"
	outboxStatusFailed      = "failed"
	outboxStatusDead        = "dead"
	maxOutboxLastErrorBytes = 4096
)

type OutboxWriter struct {
	db *DB
}

func NewOutboxWriter(db *DB) *OutboxWriter {
	return &OutboxWriter{db: db}
}

func (writer *OutboxWriter) Create(ctx context.Context, record outbox.Record) error {
	occurredAt := record.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	_, err := writer.db.executor(ctx).Exec(ctx, `
		INSERT INTO outbox_records (
			event_type,
			aggregate_type,
			aggregate_id,
			dedup_key,
			payload,
			status,
			available_at,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7, $7)
	`,
		record.EventType,
		record.AggregateType,
		record.AggregateID,
		record.DedupKey,
		record.Payload,
		outboxStatusPending,
		occurredAt,
	)
	return mapError(err)
}

func (writer *OutboxWriter) ClaimPending(ctx context.Context, batchSize int, leaseDuration time.Duration, now time.Time) ([]outbox.Record, error) {
	if batchSize <= 0 {
		return nil, nil
	}
	claimExpiresAt := now.Add(leaseDuration)
	rows, err := writer.db.executor(ctx).Query(ctx, `
		WITH claimable AS (
			SELECT id
			FROM outbox_records
			WHERE (
					status IN ($1, $2)
					AND available_at <= $3
				)
				OR (
					status = $4
					AND claim_expires_at IS NOT NULL
					AND claim_expires_at <= $3
				)
			ORDER BY available_at, created_at
			LIMIT $5
			FOR UPDATE SKIP LOCKED
		)
		UPDATE outbox_records records
		SET status = $4,
			claimed_at = $3,
			claim_expires_at = $6,
			attempts = records.attempts + 1,
			updated_at = $3
		FROM claimable
		WHERE records.id = claimable.id
		RETURNING records.id, records.event_type, records.aggregate_type, records.aggregate_id,
			records.dedup_key, records.payload, records.status, records.attempts,
			records.available_at, records.created_at, records.claimed_at,
			records.claim_expires_at, records.published_at, records.last_error,
			records.updated_at
	`, outboxStatusPending, outboxStatusFailed, now, outboxStatusProcessing, batchSize, claimExpiresAt)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	records := []outbox.Record{}
	for rows.Next() {
		record, err := scanOutboxRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return records, nil
}

func (writer *OutboxWriter) MarkPublished(ctx context.Context, recordID string, now time.Time) error {
	_, err := writer.db.executor(ctx).Exec(ctx, `
		UPDATE outbox_records
		SET status = $2,
			published_at = COALESCE(published_at, $3),
			claimed_at = NULL,
			claim_expires_at = NULL,
			updated_at = $3
		WHERE id = $1
	`, recordID, outboxStatusPublished, now)
	if err != nil {
		return mapError(err)
	}
	return writer.ensureOutboxRecordExists(ctx, recordID)
}

func (writer *OutboxWriter) MarkFailed(ctx context.Context, recordID string, errorMessage string, nextAttemptAt time.Time, now time.Time, maxAttempts int) error {
	lastError := truncateOutboxError(errorMessage)
	_, err := writer.db.executor(ctx).Exec(ctx, `
		UPDATE outbox_records
		SET status = CASE WHEN attempts >= $5 THEN $2 ELSE $3 END,
			last_error = $4,
			available_at = CASE WHEN attempts >= $5 THEN available_at ELSE $6 END,
			claimed_at = NULL,
			claim_expires_at = NULL,
			updated_at = $7
		WHERE id = $1
	`, recordID, outboxStatusDead, outboxStatusFailed, lastError, maxAttempts, nextAttemptAt, now)
	if err != nil {
		return mapError(err)
	}
	return writer.ensureOutboxRecordExists(ctx, recordID)
}

func (writer *OutboxWriter) Stats(ctx context.Context) (outbox.Stats, error) {
	rows, err := writer.db.executor(ctx).Query(ctx, `
		SELECT status, count(*)
		FROM outbox_records
		GROUP BY status
	`)
	if err != nil {
		return outbox.Stats{}, mapError(err)
	}
	defer rows.Close()

	var stats outbox.Stats
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return outbox.Stats{}, err
		}
		switch status {
		case outboxStatusPending:
			stats.Pending = count
		case outboxStatusProcessing:
			stats.Processing = count
		case outboxStatusPublished:
			stats.Published = count
		case outboxStatusFailed:
			stats.Failed = count
		case outboxStatusDead:
			stats.Dead = count
		}
	}
	if err := rows.Err(); err != nil {
		return outbox.Stats{}, mapError(err)
	}
	return stats, nil
}

func (writer *OutboxWriter) ensureOutboxRecordExists(ctx context.Context, recordID string) error {
	var exists bool
	if err := writer.db.executor(ctx).QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM outbox_records WHERE id = $1)`, recordID).Scan(&exists); err != nil {
		return mapError(err)
	}
	if !exists {
		return domain.ErrNotFound
	}
	return nil
}

func scanOutboxRecord(row pgx.Row) (outbox.Record, error) {
	var record outbox.Record
	var payload []byte
	var claimedAt sql.NullTime
	var claimExpiresAt sql.NullTime
	var publishedAt sql.NullTime
	if err := row.Scan(
		&record.ID,
		&record.EventType,
		&record.AggregateType,
		&record.AggregateID,
		&record.DedupKey,
		&payload,
		&record.Status,
		&record.Attempts,
		&record.AvailableAt,
		&record.CreatedAt,
		&claimedAt,
		&claimExpiresAt,
		&publishedAt,
		&record.LastError,
		&record.UpdatedAt,
	); err != nil {
		return outbox.Record{}, mapError(err)
	}
	record.Payload = json.RawMessage(payload)
	record.OccurredAt = record.AvailableAt
	if claimedAt.Valid {
		record.ClaimedAt = &claimedAt.Time
	}
	if claimExpiresAt.Valid {
		record.ClaimExpiresAt = &claimExpiresAt.Time
	}
	if publishedAt.Valid {
		record.PublishedAt = &publishedAt.Time
	}
	return record, nil
}

func truncateOutboxError(message string) string {
	if len(message) <= maxOutboxLastErrorBytes {
		return message
	}
	return message[:maxOutboxLastErrorBytes]
}

var _ outbox.Writer = (*OutboxWriter)(nil)
var _ outbox.Repository = (*OutboxWriter)(nil)
