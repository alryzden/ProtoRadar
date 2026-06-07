package outbox

import "context"

type Dispatcher interface {
	// Dispatch must honor ctx cancellation promptly. External dispatchers should
	// also configure their own network/client timeouts so publisher shutdown is
	// not blocked indefinitely by a stuck transport.
	Dispatch(ctx context.Context, record Record) error
}
