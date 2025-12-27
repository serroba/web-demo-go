package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/serroba/web-demo-go/internal/analytics"
	"github.com/serroba/web-demo-go/internal/analytics/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewNoop(t *testing.T) {
	logger := zap.NewNop()
	noop := store.NewNoop(logger)

	assert.NotNil(t, noop)
}

func TestNoop_SaveURLCreated(t *testing.T) {
	logger := zap.NewNop()
	noop := store.NewNoop(logger)

	event := &analytics.URLCreatedEvent{
		Code:        "abc123",
		OriginalURL: "https://example.com",
		Strategy:    "token",
		CreatedAt:   time.Now(),
	}

	err := noop.SaveURLCreated(context.Background(), event)

	require.NoError(t, err)
}

func TestNoop_SaveURLAccessed(t *testing.T) {
	logger := zap.NewNop()
	noop := store.NewNoop(logger)

	event := &analytics.URLAccessedEvent{
		Code:       "abc123",
		AccessedAt: time.Now(),
		ClientIP:   "127.0.0.1",
		UserAgent:  "TestAgent/1.0",
		Referrer:   "https://referrer.com",
	}

	err := noop.SaveURLAccessed(context.Background(), event)

	require.NoError(t, err)
}
