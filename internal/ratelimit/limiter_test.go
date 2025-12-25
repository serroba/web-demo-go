package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/serroba/web-demo-go/internal/ratelimit"
	"github.com/serroba/web-demo-go/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlidingWindowLimiter(t *testing.T) {
	t.Run("allows requests under limit", func(t *testing.T) {
		memStore := store.NewRateLimitMemoryStore()
		limiter := ratelimit.NewSlidingWindowLimiter(memStore, 5, time.Minute)

		for range 5 {
			allowed, err := limiter.Allow(context.Background(), "client1")

			require.NoError(t, err)
			assert.True(t, allowed)
		}
	})

	t.Run("denies requests over limit", func(t *testing.T) {
		memStore := store.NewRateLimitMemoryStore()
		limiter := ratelimit.NewSlidingWindowLimiter(memStore, 3, time.Minute)

		// First 3 should be allowed
		for range 3 {
			allowed, err := limiter.Allow(context.Background(), "client1")

			require.NoError(t, err)
			assert.True(t, allowed)
		}

		// 4th should be denied
		allowed, err := limiter.Allow(context.Background(), "client1")

		require.NoError(t, err)
		assert.False(t, allowed)
	})

	t.Run("tracks clients independently", func(t *testing.T) {
		memStore := store.NewRateLimitMemoryStore()
		limiter := ratelimit.NewSlidingWindowLimiter(memStore, 2, time.Minute)

		// Client 1 uses their limit
		for range 2 {
			allowed, _ := limiter.Allow(context.Background(), "client1")
			assert.True(t, allowed)
		}

		allowed, _ := limiter.Allow(context.Background(), "client1")
		assert.False(t, allowed, "client1 should be rate limited")

		// Client 2 should still be allowed
		allowed, err := limiter.Allow(context.Background(), "client2")

		require.NoError(t, err)
		assert.True(t, allowed, "client2 should still be allowed")
	})

	t.Run("allows requests after window expires", func(t *testing.T) {
		memStore := store.NewRateLimitMemoryStore()
		limiter := ratelimit.NewSlidingWindowLimiter(memStore, 2, 50*time.Millisecond)

		// Use up the limit
		for range 2 {
			allowed, _ := limiter.Allow(context.Background(), "client1")
			assert.True(t, allowed)
		}

		allowed, _ := limiter.Allow(context.Background(), "client1")
		assert.False(t, allowed, "should be rate limited")

		// Wait for window to expire
		time.Sleep(60 * time.Millisecond)

		// Should be allowed again
		allowed, err := limiter.Allow(context.Background(), "client1")

		require.NoError(t, err)
		assert.True(t, allowed, "should be allowed after window expires")
	})
}
