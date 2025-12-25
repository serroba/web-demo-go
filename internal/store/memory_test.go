package store_test

import (
	"context"
	"testing"

	"github.com/serroba/web-demo-go/internal/handlers"
	"github.com/serroba/web-demo-go/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStore_Save(t *testing.T) {
	t.Run("saves url successfully", func(t *testing.T) {
		s := store.NewMemoryStore()

		err := s.Save(context.Background(), "abc123", "https://example.com")

		require.NoError(t, err)
	})

	t.Run("overwrites existing url", func(t *testing.T) {
		s := store.NewMemoryStore()
		_ = s.Save(context.Background(), "abc123", "https://example.com")

		err := s.Save(context.Background(), "abc123", "https://other.com")
		require.NoError(t, err)

		url, _ := s.Get(context.Background(), "abc123")
		assert.Equal(t, "https://other.com", url)
	})
}

func TestMemoryStore_Get(t *testing.T) {
	t.Run("returns url when found", func(t *testing.T) {
		s := store.NewMemoryStore()
		_ = s.Save(context.Background(), "abc123", "https://example.com")

		url, err := s.Get(context.Background(), "abc123")

		require.NoError(t, err)
		assert.Equal(t, "https://example.com", url)
	})

	t.Run("returns ErrNotFound when code does not exist", func(t *testing.T) {
		s := store.NewMemoryStore()

		url, err := s.Get(context.Background(), "notfound")

		assert.Empty(t, url)
		assert.ErrorIs(t, err, handlers.ErrNotFound)
	})
}
