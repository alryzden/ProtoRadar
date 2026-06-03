package outbox

import (
	"encoding/json"
	"time"
)

type Record struct {
	AggregateType string
	AggregateID   string
	EventType     string
	DedupKey      string
	Payload       json.RawMessage
	OccurredAt    time.Time
}
