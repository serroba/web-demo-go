//go:build integration

package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/serroba/web-demo-go/internal/handlers"
	"github.com/serroba/web-demo-go/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getRedisAddr() string {
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		return addr
	}
	return "localhost:6379"
}

func TestRedisStoreIntegration(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: getRedisAddr(),
	})
	defer client.Close()

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	s := store.NewRedisStore(client)

	t.Run("save and get url", func(t *testing.T) {
		code := "testcode123"
		url := "https://example.com"

		err := s.Save(ctx, code, url)
		require.NoError(t, err)

		got, err := s.Get(ctx, code)
		require.NoError(t, err)
		assert.Equal(t, url, got)

		// Cleanup
		client.Del(ctx, "url:"+code)
	})

	t.Run("overwrite existing url", func(t *testing.T) {
		code := "overwrite123"
		_ = s.Save(ctx, code, "https://old.com")

		err := s.Save(ctx, code, "https://new.com")
		require.NoError(t, err)

		got, _ := s.Get(ctx, code)
		assert.Equal(t, "https://new.com", got)

		// Cleanup
		client.Del(ctx, "url:"+code)
	})

	t.Run("get non-existent returns ErrNotFound", func(t *testing.T) {
		url, err := s.Get(ctx, "nonexistent")

		assert.Empty(t, url)
		assert.ErrorIs(t, err, handlers.ErrNotFound)
	})
}
