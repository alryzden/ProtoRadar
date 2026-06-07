package outbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"
)

type PublisherOptions struct {
	Enabled               bool
	BatchSize             int
	PollInterval          time.Duration
	LeaseDuration         time.Duration
	MaxAttempts           int
	InitialRetryBackoff   time.Duration
	MaxRetryBackoff       time.Duration
	ErrorMessageMaxLength int
	Logger                *slog.Logger
	Observer              PublisherObserver
	Clock                 Clock
}

// PublisherObserver lets bootstrap connect publisher counters to an application metrics registry without coupling outbox to transport packages.
type PublisherObserver interface {
	RecordClaimed(count int)
	RecordDispatch(status string, duration time.Duration)
	RecordError(status string)
	RecordStats(stats Stats)
}

type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}

type Publisher struct {
	repository Repository
	dispatcher Dispatcher
	options    PublisherOptions
}

// DefaultPublisherOptions returns safe Community defaults. The publisher is disabled by default
// so existing installations do not drain durable outbox records until explicitly wired and enabled.
func DefaultPublisherOptions() PublisherOptions {
	return PublisherOptions{
		Enabled:               false,
		BatchSize:             50,
		PollInterval:          5 * time.Second,
		LeaseDuration:         30 * time.Second,
		MaxAttempts:           5,
		InitialRetryBackoff:   30 * time.Second,
		MaxRetryBackoff:       5 * time.Minute,
		ErrorMessageMaxLength: 4096,
		Clock:                 SystemClock{},
	}
}

func NewPublisher(repository Repository, dispatcher Dispatcher, options PublisherOptions) (*Publisher, error) {
	options = normalizePublisherOptions(options)
	if repository == nil {
		return nil, fmt.Errorf("outbox publisher repository is required")
	}
	if dispatcher == nil {
		return nil, fmt.Errorf("outbox publisher dispatcher is required")
	}
	return &Publisher{repository: repository, dispatcher: dispatcher, options: options}, nil
}

func (publisher *Publisher) Run(ctx context.Context) error {
	if !publisher.options.Enabled {
		publisher.logInfo(ctx, "outbox_publisher_disabled")
		return nil
	}

	publisher.logInfo(ctx, "outbox_publisher_started")
	defer publisher.logInfo(ctx, "outbox_publisher_stopped")

	if err := publisher.PublishOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		publisher.logError(ctx, "outbox_publisher_iteration_failed", err)
	}

	ticker := time.NewTicker(publisher.options.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := publisher.PublishOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
				publisher.logError(ctx, "outbox_publisher_iteration_failed", err)
			}
		}
	}
}

func (publisher *Publisher) PublishOnce(ctx context.Context) error {
	if !publisher.options.Enabled {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	now := publisher.options.Clock.Now()
	records, err := publisher.repository.ClaimPending(ctx, publisher.options.BatchSize, publisher.options.LeaseDuration, now)
	if err != nil {
		publisher.recordError("claim_failed")
		return err
	}
	publisher.recordClaimed(len(records))
	publisher.recordStats(ctx)

	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := publisher.dispatchRecord(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

func (publisher *Publisher) dispatchRecord(ctx context.Context, record Record) error {
	started := publisher.options.Clock.Now()
	err := publisher.dispatcher.Dispatch(ctx, record)
	duration := publisher.options.Clock.Now().Sub(started)
	if err == nil {
		if markErr := publisher.repository.MarkPublished(ctx, record.ID, publisher.options.Clock.Now()); markErr != nil {
			publisher.recordError("mark_published_failed")
			publisher.logError(ctx, "outbox_mark_published_failed", markErr, recordLogAttrs(record)...)
			return markErr
		}
		publisher.recordDispatch("published", duration)
		publisher.recordStats(ctx)
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}

	now := publisher.options.Clock.Now()
	nextAttemptAt := now.Add(publisher.retryBackoff(record.Attempts))
	message := truncateMessage(err.Error(), publisher.options.ErrorMessageMaxLength)
	if markErr := publisher.repository.MarkFailed(ctx, record.ID, message, nextAttemptAt, now, publisher.options.MaxAttempts); markErr != nil {
		publisher.recordError("mark_failed_failed")
		publisher.logError(ctx, "outbox_mark_failed_failed", markErr, recordLogAttrs(record)...)
		return markErr
	}
	status := "failed"
	if record.Attempts >= publisher.options.MaxAttempts {
		status = "dead"
		publisher.logError(ctx, "outbox_record_marked_dead", err, recordLogAttrs(record)...)
	}
	publisher.recordDispatch(status, duration)
	publisher.recordStats(ctx)
	publisher.logError(ctx, "outbox_dispatch_failed", err, recordLogAttrs(record)...)
	return nil
}

func (publisher *Publisher) retryBackoff(attempts int) time.Duration {
	attempt := attempts
	if attempt < 1 {
		attempt = 1
	}
	multiplier := math.Pow(2, float64(attempt-1))
	backoff := time.Duration(float64(publisher.options.InitialRetryBackoff) * multiplier)
	if backoff <= 0 || backoff > publisher.options.MaxRetryBackoff {
		return publisher.options.MaxRetryBackoff
	}
	return backoff
}

func normalizePublisherOptions(options PublisherOptions) PublisherOptions {
	defaults := DefaultPublisherOptions()
	if options.BatchSize <= 0 {
		options.BatchSize = defaults.BatchSize
	}
	if options.PollInterval <= 0 {
		options.PollInterval = defaults.PollInterval
	}
	if options.LeaseDuration <= 0 {
		options.LeaseDuration = defaults.LeaseDuration
	}
	if options.MaxAttempts <= 0 {
		options.MaxAttempts = defaults.MaxAttempts
	}
	if options.InitialRetryBackoff <= 0 {
		options.InitialRetryBackoff = defaults.InitialRetryBackoff
	}
	if options.MaxRetryBackoff <= 0 {
		options.MaxRetryBackoff = defaults.MaxRetryBackoff
	}
	if options.MaxRetryBackoff < options.InitialRetryBackoff {
		options.MaxRetryBackoff = options.InitialRetryBackoff
	}
	if options.ErrorMessageMaxLength <= 0 {
		options.ErrorMessageMaxLength = defaults.ErrorMessageMaxLength
	}
	if options.Clock == nil {
		options.Clock = defaults.Clock
	}
	return options
}

func (publisher *Publisher) recordClaimed(count int) {
	if publisher.options.Observer != nil {
		publisher.options.Observer.RecordClaimed(count)
	}
}

func (publisher *Publisher) recordDispatch(status string, duration time.Duration) {
	if publisher.options.Observer != nil {
		publisher.options.Observer.RecordDispatch(status, duration)
	}
}

func (publisher *Publisher) recordError(status string) {
	if publisher.options.Observer != nil {
		publisher.options.Observer.RecordError(status)
	}
}

func (publisher *Publisher) recordStats(ctx context.Context) {
	if publisher.options.Observer == nil {
		return
	}
	stats, err := publisher.repository.Stats(ctx)
	if err != nil {
		publisher.recordError("stats_failed")
		publisher.logError(ctx, "outbox_stats_failed", err)
		return
	}
	publisher.options.Observer.RecordStats(stats)
}

func (publisher *Publisher) logInfo(ctx context.Context, message string, attrs ...slog.Attr) {
	if publisher.options.Logger == nil {
		return
	}
	publisher.options.Logger.LogAttrs(ctx, slog.LevelInfo, message, attrs...)
}

func (publisher *Publisher) logError(ctx context.Context, message string, err error, attrs ...slog.Attr) {
	if publisher.options.Logger == nil {
		return
	}
	allAttrs := append([]slog.Attr{slog.String("error", err.Error())}, attrs...)
	publisher.options.Logger.LogAttrs(ctx, slog.LevelError, message, allAttrs...)
}

func recordLogAttrs(record Record) []slog.Attr {
	return []slog.Attr{
		slog.String("record_id", record.ID),
		slog.String("event_type", record.EventType),
		slog.String("aggregate_type", record.AggregateType),
		slog.String("aggregate_id", record.AggregateID),
		slog.String("dedup_key", record.DedupKey),
		slog.Int("attempts", record.Attempts),
		slog.Int("payload_bytes", len(record.Payload)),
	}
}

func truncateMessage(message string, maxLength int) string {
	if maxLength <= 0 || len(message) <= maxLength {
		return message
	}
	return message[:maxLength]
}
