package handlers_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/serroba/web-demo-go/internal/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockHealthChecker struct {
	err error
}

func (m *mockHealthChecker) Ping(_ context.Context) error {
	return m.err
}

func TestNewHealthHandler(t *testing.T) {
	checker := &mockHealthChecker{}
	handler := handlers.NewHealthHandler(checker)

	assert.NotNil(t, handler)
}

func TestHealthHandler_Check(t *testing.T) {
	t.Run("returns ok when redis is healthy", func(t *testing.T) {
		checker := &mockHealthChecker{err: nil}
		handler := handlers.NewHealthHandler(checker)

		resp, err := handler.Check(context.Background(), nil)

		require.NoError(t, err)
		assert.Equal(t, "ok", resp.Body.Status)
		assert.Equal(t, "healthy", resp.Body.Redis)
	})

	t.Run("returns degraded when redis is unhealthy", func(t *testing.T) {
		checker := &mockHealthChecker{err: errors.New("connection refused")}
		handler := handlers.NewHealthHandler(checker)

		resp, err := handler.Check(context.Background(), nil)

		require.NoError(t, err)
		assert.Equal(t, "degraded", resp.Body.Status)
		assert.Equal(t, "unhealthy", resp.Body.Redis)
	})
}

func TestRedisHealthChecker(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available at %s: %v", addr, err)
	}

	t.Run("NewRedisHealthChecker creates checker", func(t *testing.T) {
		checker := handlers.NewRedisHealthChecker(client)

		assert.NotNil(t, checker)
	})

	t.Run("Ping returns nil when redis is available", func(t *testing.T) {
		checker := handlers.NewRedisHealthChecker(client)

		err := checker.Ping(context.Background())

		assert.NoError(t, err)
	})
}
