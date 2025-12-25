package health_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/serroba/web-demo-go/internal/health"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockChecker struct {
	err error
}

func (m *mockChecker) Ping(_ context.Context) error {
	return m.err
}

func TestNewHandler(t *testing.T) {
	checker := &mockChecker{}
	handler := health.NewHandler(checker)

	assert.NotNil(t, handler)
}

func TestHandler_Check(t *testing.T) {
	t.Run("returns ok when redis is healthy", func(t *testing.T) {
		checker := &mockChecker{err: nil}
		handler := health.NewHandler(checker)

		resp, err := handler.Check(context.Background(), nil)

		require.NoError(t, err)
		assert.Equal(t, "ok", resp.Body.Status)
		assert.Equal(t, "healthy", resp.Body.Redis)
	})

	t.Run("returns degraded when redis is unhealthy", func(t *testing.T) {
		checker := &mockChecker{err: errors.New("connection refused")}
		handler := health.NewHandler(checker)

		resp, err := handler.Check(context.Background(), nil)

		require.NoError(t, err)
		assert.Equal(t, "degraded", resp.Body.Status)
		assert.Equal(t, "unhealthy", resp.Body.Redis)
	})
}

func TestRedisChecker(t *testing.T) {
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

	t.Run("NewRedisChecker creates checker", func(t *testing.T) {
		checker := health.NewRedisChecker(client)

		assert.NotNil(t, checker)
	})

	t.Run("Ping returns nil when redis is available", func(t *testing.T) {
		checker := health.NewRedisChecker(client)

		err := checker.Ping(context.Background())

		assert.NoError(t, err)
	})
}
