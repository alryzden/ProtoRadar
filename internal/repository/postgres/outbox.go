package postgres

import (
	"context"
	"time"

	"github.com/alryzden/ProtoRadar/internal/outbox"
)

const outboxStatusPending = "pending"

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
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
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

var _ outbox.Writer = (*OutboxWriter)(nil)
