package ratelimit

import (
	"context"
	"time"
)

// Limiter defines the interface for rate limiting.
type Limiter interface {
	// Allow checks if a request from the given key should be allowed.
	Allow(ctx context.Context, key string) (allowed bool, err error)
}

// SlidingWindowLimiter implements rate limiting using a sliding window algorithm.
type SlidingWindowLimiter struct {
	store  Store
	limit  int64
	window time.Duration
}

// NewSlidingWindowLimiter creates a new sliding window rate limiter.
func NewSlidingWindowLimiter(store Store, limit int64, window time.Duration) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		store:  store,
		limit:  limit,
		window: window,
	}
}

func (l *SlidingWindowLimiter) Allow(ctx context.Context, key string) (bool, error) {
	count, err := l.store.Record(ctx, key, l.window)
	if err != nil {
		return false, err
	}

	return count <= l.limit, nil
}
