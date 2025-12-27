//go:build integration

package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/serroba/web-demo-go/internal/shortener"
	"github.com/serroba/web-demo-go/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getDatabaseURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable"
}

func TestPostgresStoreIntegration(t *testing.T) {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, getDatabaseURL())
	if err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}

	s := store.NewPostgresStore(pool)

	t.Run("save and get by code", func(t *testing.T) {
		shortURL := &shortener.ShortURL{
			Code:        shortener.Code("pgtestcode1"),
			OriginalURL: "https://example.com",
			CreatedAt:   time.Now().UTC().Truncate(time.Microsecond),
		}

		err := s.Save(ctx, shortURL)
		require.NoError(t, err)

		got, err := s.GetByCode(ctx, shortURL.Code)
		require.NoError(t, err)
		assert.Equal(t, shortURL.OriginalURL, got.OriginalURL)
		assert.Equal(t, shortURL.Code, got.Code)

		// Cleanup
		_, _ = pool.Exec(ctx, "DELETE FROM short_urls WHERE code = $1", string(shortURL.Code))
	})

	t.Run("save and get by hash", func(t *testing.T) {
		shortURL := &shortener.ShortURL{
			Code:        shortener.Code("pghashcode1"),
			OriginalURL: "https://example.com/hashed",
			URLHash:     shortener.URLHash("pgabc123hash"),
			CreatedAt:   time.Now().UTC().Truncate(time.Microsecond),
		}

		err := s.Save(ctx, shortURL)
		require.NoError(t, err)

		got, err := s.GetByHash(ctx, shortURL.URLHash)
		require.NoError(t, err)
		assert.Equal(t, shortURL.OriginalURL, got.OriginalURL)
		assert.Equal(t, shortURL.Code, got.Code)
		assert.Equal(t, shortURL.URLHash, got.URLHash)

		// Cleanup
		_, _ = pool.Exec(ctx, "DELETE FROM short_urls WHERE code = $1", string(shortURL.Code))
	})

	t.Run("save with ON CONFLICT does not error", func(t *testing.T) {
		code := shortener.Code("pgconflict1")
		first := &shortener.ShortURL{
			Code:        code,
			OriginalURL: "https://old.com",
			CreatedAt:   time.Now().UTC().Truncate(time.Microsecond),
		}
		second := &shortener.ShortURL{
			Code:        code,
			OriginalURL: "https://new.com",
			CreatedAt:   time.Now().UTC().Truncate(time.Microsecond),
		}

		err := s.Save(ctx, first)
		require.NoError(t, err)

		// Second save should not error (ON CONFLICT DO NOTHING)
		err = s.Save(ctx, second)
		require.NoError(t, err)

		// First value should be preserved
		got, _ := s.GetByCode(ctx, code)
		assert.Equal(t, "https://old.com", got.OriginalURL)

		// Cleanup
		_, _ = pool.Exec(ctx, "DELETE FROM short_urls WHERE code = $1", string(code))
	})

	t.Run("get non-existent returns ErrNotFound", func(t *testing.T) {
		got, err := s.GetByCode(ctx, "pgnonexistent")

		assert.Nil(t, got)
		assert.ErrorIs(t, err, shortener.ErrNotFound)
	})

	t.Run("get by hash non-existent returns ErrNotFound", func(t *testing.T) {
		got, err := s.GetByHash(ctx, "pgnonexistenthash")

		assert.Nil(t, got)
		assert.ErrorIs(t, err, shortener.ErrNotFound)
	})
}
