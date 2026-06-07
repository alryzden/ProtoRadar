package outbox

import (
	"context"
	"time"
)

type Repository interface {
	Writer
	ClaimPending(ctx context.Context, batchSize int, leaseDuration time.Duration, now time.Time) ([]Record, error)
	MarkPublished(ctx context.Context, recordID string, now time.Time) error
	MarkFailed(ctx context.Context, recordID string, errorMessage string, nextAttemptAt time.Time, now time.Time, maxAttempts int) error
	Stats(ctx context.Context) (Stats, error)
}

type Stats struct {
	Pending    int
	Processing int
	Published  int
	Failed     int
	Dead       int
}
