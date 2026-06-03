package outbox

import "context"

type Writer interface {
	Create(ctx context.Context, record Record) error
}
