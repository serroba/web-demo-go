//go:build integration

package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/serroba/web-demo-go/internal/domain"
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

	t.Run("save and get by code", func(t *testing.T) {
		shortURL := &domain.ShortURL{
			Code:        "testcode123",
			OriginalURL: "https://example.com",
		}

		err := s.Save(ctx, shortURL)
		require.NoError(t, err)

		got, err := s.GetByCode(ctx, shortURL.Code)
		require.NoError(t, err)
		assert.Equal(t, shortURL.OriginalURL, got.OriginalURL)
		assert.Equal(t, shortURL.Code, got.Code)

		// Cleanup
		client.Del(ctx, "url:"+string(shortURL.Code))
	})

	t.Run("save and get by hash", func(t *testing.T) {
		shortURL := &domain.ShortURL{
			Code:        "hashcode123",
			OriginalURL: "https://example.com/hashed",
			URLHash:     "abc123hash",
		}

		err := s.Save(ctx, shortURL)
		require.NoError(t, err)

		got, err := s.GetByHash(ctx, shortURL.URLHash)
		require.NoError(t, err)
		assert.Equal(t, shortURL.OriginalURL, got.OriginalURL)
		assert.Equal(t, shortURL.Code, got.Code)
		assert.Equal(t, shortURL.URLHash, got.URLHash)

		// Cleanup
		client.Del(ctx, "url:"+string(shortURL.Code))
		client.HDel(ctx, "url_hashes", string(shortURL.URLHash))
	})

	t.Run("overwrite existing url", func(t *testing.T) {
		code := domain.Code("overwrite123")
		_ = s.Save(ctx, &domain.ShortURL{Code: code, OriginalURL: "https://old.com"})

		err := s.Save(ctx, &domain.ShortURL{Code: code, OriginalURL: "https://new.com"})
		require.NoError(t, err)

		got, _ := s.GetByCode(ctx, code)
		assert.Equal(t, "https://new.com", got.OriginalURL)

		// Cleanup
		client.Del(ctx, "url:"+string(code))
	})

	t.Run("get non-existent returns ErrNotFound", func(t *testing.T) {
		got, err := s.GetByCode(ctx, "nonexistent")

		assert.Nil(t, got)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("get by hash non-existent returns ErrNotFound", func(t *testing.T) {
		got, err := s.GetByHash(ctx, "nonexistenthash")

		assert.Nil(t, got)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
