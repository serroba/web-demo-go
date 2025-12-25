package ratelimit

import (
	"context"
	"time"
)

// Store defines the interface for rate limit data storage.
type Store interface {
	// Record records a request and returns the count of requests in the current window.
	// It automatically prunes expired entries.
	Record(ctx context.Context, key string, window time.Duration) (count int64, err error)
}
