package store

import (
	"context"
	"sync"
	"time"
)

// RateLimitMemoryStore is an in-memory implementation of ratelimit.Store.
type RateLimitMemoryStore struct {
	mu       sync.Mutex
	requests map[string][]time.Time
}

// NewRateLimitMemoryStore creates a new in-memory rate limit store.
func NewRateLimitMemoryStore() *RateLimitMemoryStore {
	return &RateLimitMemoryStore{
		requests: make(map[string][]time.Time),
	}
}

func (s *RateLimitMemoryStore) Record(_ context.Context, key string, window time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-window)

	// Get existing timestamps and prune expired ones
	timestamps := s.requests[key]
	valid := make([]time.Time, 0, len(timestamps)+1)

	for _, ts := range timestamps {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}

	// Add current request
	valid = append(valid, now)
	s.requests[key] = valid

	return int64(len(valid)), nil
}
