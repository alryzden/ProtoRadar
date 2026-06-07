package outbox

import (
	"bytes"
	"context"
	"errors"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPublisherClaimsDispatchesAndMarksPublished(t *testing.T) {
	repo := newFakeRepository([]Record{testRecord("record-1", 0)})
	dispatcher := &fakeDispatcher{}
	observer := &fakeObserver{}
	publisher := newTestPublisher(t, repo, dispatcher, PublisherOptions{Observer: observer})

	if err := publisher.PublishOnce(context.Background()); err != nil {
		t.Fatalf("publish once: %v", err)
	}

	if len(dispatcher.records) != 1 || dispatcher.records[0].ID != "record-1" {
		t.Fatalf("dispatched = %#v", dispatcher.records)
	}
	if len(repo.published) != 1 || repo.published[0] != "record-1" {
		t.Fatalf("published = %#v", repo.published)
	}
	if len(repo.failed) != 0 {
		t.Fatalf("failed = %#v", repo.failed)
	}
	if observer.claimed != 1 || observer.dispatchStatus["published"] != 1 {
		t.Fatalf("observer = %#v", observer)
	}
}

func TestPublisherMarksFailedWithRetryOnDispatcherError(t *testing.T) {
	repo := newFakeRepository([]Record{testRecord("record-1", 0)})
	dispatcher := &fakeDispatcher{err: errors.New("temporary failure with secret-free details")}
	publisher := newTestPublisher(t, repo, dispatcher, PublisherOptions{
		InitialRetryBackoff:   time.Minute,
		MaxRetryBackoff:       10 * time.Minute,
		ErrorMessageMaxLength: 20,
	})

	if err := publisher.PublishOnce(context.Background()); err != nil {
		t.Fatalf("publish once: %v", err)
	}

	if len(repo.failed) != 1 {
		t.Fatalf("failed = %#v", repo.failed)
	}
	failure := repo.failed[0]
	if failure.id != "record-1" || failure.maxAttempts != 5 {
		t.Fatalf("failure = %#v", failure)
	}
	if failure.message != "temporary failure wi" {
		t.Fatalf("message = %q", failure.message)
	}
	if !failure.nextAttemptAt.Equal(repo.clock.now.Add(time.Minute)) {
		t.Fatalf("next attempt = %s, want %s", failure.nextAttemptAt, repo.clock.now.Add(time.Minute))
	}
	if len(repo.published) != 0 {
		t.Fatalf("published = %#v", repo.published)
	}
}

func TestPublisherRepeatedFailuresEventuallyMarkDead(t *testing.T) {
	repo := newFakeRepository([]Record{testRecord("record-1", 0)})
	dispatcher := &fakeDispatcher{err: errors.New("temporary failure")}
	publisher := newTestPublisher(t, repo, dispatcher, PublisherOptions{MaxAttempts: 3, InitialRetryBackoff: time.Second, MaxRetryBackoff: 10 * time.Second})

	for i := 0; i < 3; i++ {
		if err := publisher.PublishOnce(context.Background()); err != nil {
			t.Fatalf("publish once %d: %v", i, err)
		}
	}

	if len(repo.failed) != 3 {
		t.Fatalf("failed calls = %#v", repo.failed)
	}
	if state := repo.records["record-1"]; state.Status != "dead" || state.Attempts != 3 {
		t.Fatalf("record state = %#v", state)
	}
	if len(dispatcher.records) != 3 {
		t.Fatalf("dispatch count = %d, want 3", len(dispatcher.records))
	}
}

func TestPublisherStopsOnContextCancellationWithoutMarking(t *testing.T) {
	repo := newFakeRepository([]Record{testRecord("record-1", 0), testRecord("record-2", 0)})
	ctx, cancel := context.WithCancel(context.Background())
	dispatcher := &fakeDispatcher{onDispatch: func(_ Record) error {
		cancel()
		return context.Canceled
	}}
	publisher := newTestPublisher(t, repo, dispatcher, PublisherOptions{})

	if err := publisher.PublishOnce(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
	if len(repo.published) != 0 || len(repo.failed) != 0 {
		t.Fatalf("published/failed = %#v/%#v", repo.published, repo.failed)
	}
	if len(dispatcher.records) != 1 {
		t.Fatalf("dispatch count = %d, want 1", len(dispatcher.records))
	}
}

func TestPublisherDisabledDoesNotClaimOrDispatch(t *testing.T) {
	repo := newFakeRepository([]Record{testRecord("record-1", 0)})
	dispatcher := &fakeDispatcher{}
	publisher, err := NewPublisher(repo, dispatcher, PublisherOptions{Enabled: false})
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}

	if err := publisher.PublishOnce(context.Background()); err != nil {
		t.Fatalf("publish once: %v", err)
	}
	if repo.claimCalls != 0 || len(dispatcher.records) != 0 {
		t.Fatalf("claim calls/dispatched = %d/%d", repo.claimCalls, len(dispatcher.records))
	}
}

func TestPublisherRespectsBatchSize(t *testing.T) {
	repo := newFakeRepository([]Record{testRecord("record-1", 0), testRecord("record-2", 0), testRecord("record-3", 0)})
	dispatcher := &fakeDispatcher{}
	publisher := newTestPublisher(t, repo, dispatcher, PublisherOptions{BatchSize: 2})

	if err := publisher.PublishOnce(context.Background()); err != nil {
		t.Fatalf("publish once: %v", err)
	}
	if repo.lastBatchSize != 2 {
		t.Fatalf("batch size = %d, want 2", repo.lastBatchSize)
	}
	if len(dispatcher.records) != 2 {
		t.Fatalf("dispatch count = %d, want 2", len(dispatcher.records))
	}
}

func TestPublisherRunStopsOnContextCancellation(t *testing.T) {
	repo := newFakeRepository(nil)
	dispatcher := &fakeDispatcher{}
	publisher := newTestPublisher(t, repo, dispatcher, PublisherOptions{PollInterval: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := publisher.Run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestLoggingDispatcherDoesNotLogPayloadSecrets(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	dispatcher, err := NewLoggingDispatcher(logger)
	if err != nil {
		t.Fatalf("new logging dispatcher: %v", err)
	}
	record := testRecord("record-1", 1)
	record.Payload = []byte(`{"api_token":"secret-token-value"}`)

	if err := dispatcher.Dispatch(context.Background(), record); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	text := output.String()
	if strings.Contains(text, "secret-token-value") || strings.Contains(text, "api_token") {
		t.Fatalf("log leaked payload: %s", text)
	}
	for _, want := range []string{"outbox_record_dispatched_locally", "record-1", "protoradar.test.created", "payload_bytes"} {
		if !strings.Contains(text, want) {
			t.Fatalf("log missing %q: %s", want, text)
		}
	}
}

func TestOutboxPackageDoesNotImportKafkaOrSarama(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read outbox dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", entry.Name(), err)
		}
		for _, imported := range parsed.Imports {
			path := strings.ToLower(strings.Trim(imported.Path.Value, "\""))
			if strings.Contains(path, "sarama") || strings.Contains(path, "kafka") {
				t.Fatalf("%s imports forbidden package %q", entry.Name(), path)
			}
		}
	}
}

func newTestPublisher(t *testing.T, repo *fakeRepository, dispatcher *fakeDispatcher, options PublisherOptions) *Publisher {
	t.Helper()
	options.Enabled = true
	if options.Clock == nil {
		options.Clock = repo.clock
	}
	publisher, err := NewPublisher(repo, dispatcher, options)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	return publisher
}

func testRecord(id string, attempts int) Record {
	return Record{
		ID:            id,
		AggregateType: "test",
		AggregateID:   "aggregate-" + id,
		EventType:     "protoradar.test.created",
		DedupKey:      "dedup-" + id,
		Payload:       []byte(`{"ok":true}`),
		Status:        "pending",
		Attempts:      attempts,
	}
}

type fakeRepository struct {
	records       map[string]Record
	order         []string
	clock         *fakeClock
	claimCalls    int
	lastBatchSize int
	published     []string
	failed        []fakeFailure
}

type fakeFailure struct {
	id            string
	message       string
	nextAttemptAt time.Time
	maxAttempts   int
}

func newFakeRepository(records []Record) *fakeRepository {
	repo := &fakeRepository{
		records: map[string]Record{},
		clock:   &fakeClock{now: time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)},
	}
	for _, record := range records {
		repo.records[record.ID] = record
		repo.order = append(repo.order, record.ID)
	}
	return repo
}

func (repo *fakeRepository) Create(_ context.Context, record Record) error {
	repo.records[record.ID] = record
	repo.order = append(repo.order, record.ID)
	return nil
}

func (repo *fakeRepository) ClaimPending(_ context.Context, batchSize int, leaseDuration time.Duration, now time.Time) ([]Record, error) {
	repo.claimCalls++
	repo.lastBatchSize = batchSize
	claimed := []Record{}
	for _, id := range repo.order {
		if len(claimed) >= batchSize {
			break
		}
		record := repo.records[id]
		if record.Status != "pending" && record.Status != "failed" {
			continue
		}
		record.Status = "processing"
		record.Attempts++
		claimedAt := now
		claimExpiresAt := now.Add(leaseDuration)
		record.ClaimedAt = &claimedAt
		record.ClaimExpiresAt = &claimExpiresAt
		repo.records[id] = record
		claimed = append(claimed, record)
	}
	return claimed, nil
}

func (repo *fakeRepository) MarkPublished(_ context.Context, recordID string, now time.Time) error {
	record := repo.records[recordID]
	record.Status = "published"
	record.PublishedAt = &now
	repo.records[recordID] = record
	repo.published = append(repo.published, recordID)
	return nil
}

func (repo *fakeRepository) MarkFailed(_ context.Context, recordID string, errorMessage string, nextAttemptAt time.Time, _ time.Time, maxAttempts int) error {
	record := repo.records[recordID]
	if record.Attempts >= maxAttempts {
		record.Status = "dead"
	} else {
		record.Status = "failed"
	}
	record.LastError = errorMessage
	record.AvailableAt = nextAttemptAt
	record.ClaimedAt = nil
	record.ClaimExpiresAt = nil
	repo.records[recordID] = record
	repo.failed = append(repo.failed, fakeFailure{id: recordID, message: errorMessage, nextAttemptAt: nextAttemptAt, maxAttempts: maxAttempts})
	return nil
}

func (repo *fakeRepository) Stats(context.Context) (Stats, error) {
	var stats Stats
	for _, record := range repo.records {
		switch record.Status {
		case "pending":
			stats.Pending++
		case "processing":
			stats.Processing++
		case "published":
			stats.Published++
		case "failed":
			stats.Failed++
		case "dead":
			stats.Dead++
		}
	}
	return stats, nil
}

type fakeDispatcher struct {
	records    []Record
	err        error
	onDispatch func(record Record) error
}

func (dispatcher *fakeDispatcher) Dispatch(_ context.Context, record Record) error {
	dispatcher.records = append(dispatcher.records, record)
	if dispatcher.onDispatch != nil {
		return dispatcher.onDispatch(record)
	}
	return dispatcher.err
}

type fakeClock struct {
	now time.Time
}

func (clock *fakeClock) Now() time.Time {
	return clock.now
}

type fakeObserver struct {
	claimed        int
	dispatchStatus map[string]int
	errorStatus    map[string]int
	stats          Stats
}

func (observer *fakeObserver) RecordClaimed(count int) {
	observer.claimed += count
}

func (observer *fakeObserver) RecordDispatch(status string, _ time.Duration) {
	if observer.dispatchStatus == nil {
		observer.dispatchStatus = map[string]int{}
	}
	observer.dispatchStatus[status]++
}

func (observer *fakeObserver) RecordError(status string) {
	if observer.errorStatus == nil {
		observer.errorStatus = map[string]int{}
	}
	observer.errorStatus[status]++
}

func (observer *fakeObserver) RecordStats(stats Stats) {
	observer.stats = stats
}
