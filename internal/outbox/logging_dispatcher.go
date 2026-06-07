package outbox

import (
	"context"
	"fmt"
	"log/slog"
)

// LoggingDispatcher is the Community dispatcher: it records a local structured log entry
// and does not deliver to Kafka, Sarama, webhooks, or any external event bus.
type LoggingDispatcher struct {
	logger *slog.Logger
}

func NewLoggingDispatcher(logger *slog.Logger) (*LoggingDispatcher, error) {
	if logger == nil {
		return nil, fmt.Errorf("outbox logging dispatcher logger is required")
	}
	return &LoggingDispatcher{logger: logger}, nil
}

func (dispatcher *LoggingDispatcher) Dispatch(ctx context.Context, record Record) error {
	dispatcher.logger.InfoContext(ctx, "outbox_record_dispatched_locally",
		slog.String("record_id", record.ID),
		slog.String("event_type", record.EventType),
		slog.String("aggregate_type", record.AggregateType),
		slog.String("aggregate_id", record.AggregateID),
		slog.String("dedup_key", record.DedupKey),
		slog.Int("payload_bytes", len(record.Payload)),
	)
	return nil
}

var _ Dispatcher = (*LoggingDispatcher)(nil)
