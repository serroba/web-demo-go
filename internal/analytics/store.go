package analytics

import "context"

// Store defines the interface for persisting analytics events.
type Store interface {
	SaveURLCreated(ctx context.Context, event *URLCreatedEvent) error
	SaveURLAccessed(ctx context.Context, event *URLAccessedEvent) error
}
