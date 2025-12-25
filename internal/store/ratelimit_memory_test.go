package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/serroba/web-demo-go/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimitMemoryStore(t *testing.T) {
	t.Run("records and counts requests", func(t *testing.T) {
		s := store.NewRateLimitMemoryStore()

		count1, err := s.Record(context.Background(), "key1", time.Minute)

		require.NoError(t, err)
		assert.Equal(t, int64(1), count1)

		count2, err := s.Record(context.Background(), "key1", time.Minute)

		require.NoError(t, err)
		assert.Equal(t, int64(2), count2)

		count3, err := s.Record(context.Background(), "key1", time.Minute)

		require.NoError(t, err)
		assert.Equal(t, int64(3), count3)
	})

	t.Run("tracks keys independently", func(t *testing.T) {
		s := store.NewRateLimitMemoryStore()

		_, _ = s.Record(context.Background(), "key1", time.Minute)
		_, _ = s.Record(context.Background(), "key1", time.Minute)

		count, err := s.Record(context.Background(), "key2", time.Minute)

		require.NoError(t, err)
		assert.Equal(t, int64(1), count, "key2 should have its own counter")
	})

	t.Run("prunes expired entries", func(t *testing.T) {
		s := store.NewRateLimitMemoryStore()

		// Record some requests
		_, _ = s.Record(context.Background(), "key1", 50*time.Millisecond)
		_, _ = s.Record(context.Background(), "key1", 50*time.Millisecond)

		// Wait for them to expire
		time.Sleep(60 * time.Millisecond)

		// New request should only count itself
		count, err := s.Record(context.Background(), "key1", 50*time.Millisecond)

		require.NoError(t, err)
		assert.Equal(t, int64(1), count, "expired entries should be pruned")
	})
}
