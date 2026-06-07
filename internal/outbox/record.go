package outbox

import (
	"encoding/json"
	"time"
)

type Record struct {
	ID             string
	AggregateType  string
	AggregateID    string
	EventType      string
	DedupKey       string
	Payload        json.RawMessage
	Status         string
	Attempts       int
	AvailableAt    time.Time
	CreatedAt      time.Time
	ClaimedAt      *time.Time
	ClaimExpiresAt *time.Time
	PublishedAt    *time.Time
	LastError      string
	UpdatedAt      time.Time
	OccurredAt     time.Time
}
