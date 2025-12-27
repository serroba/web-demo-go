package messaging_test

import (
	"context"
	"errors"
	"testing"

	"github.com/serroba/web-demo-go/internal/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type mockRunnable struct {
	started     bool
	shutdown    bool
	startErr    error
	shutdownErr error
}

func (m *mockRunnable) Start(_ context.Context) error {
	if m.startErr != nil {
		return m.startErr
	}

	m.started = true

	return nil
}

func (m *mockRunnable) Shutdown() error {
	m.shutdown = true

	return m.shutdownErr
}

func TestConsumerGroup_Start(t *testing.T) {
	t.Run("starts all consumers", func(t *testing.T) {
		sub := newMockSubscriber()
		group := messaging.NewConsumerGroup(sub, zap.NewNop())
		consumer1 := &mockRunnable{}
		consumer2 := &mockRunnable{}

		group.Add(consumer1)
		group.Add(consumer2)

		err := group.Start(context.Background())

		require.NoError(t, err)
		assert.True(t, consumer1.started)
		assert.True(t, consumer2.started)
	})

	t.Run("rolls back on failure", func(t *testing.T) {
		sub := newMockSubscriber()
		group := messaging.NewConsumerGroup(sub, zap.NewNop())
		consumer1 := &mockRunnable{}
		consumer2 := &mockRunnable{startErr: errors.New("start error")}

		group.Add(consumer1)
		group.Add(consumer2)

		err := group.Start(context.Background())

		require.Error(t, err)
		assert.True(t, consumer1.started)
		assert.True(t, consumer1.shutdown) // Should be rolled back
		assert.False(t, consumer2.started)
	})
}

func TestConsumerGroup_Shutdown(t *testing.T) {
	t.Run("shuts down all consumers", func(t *testing.T) {
		sub := newMockSubscriber()
		group := messaging.NewConsumerGroup(sub, zap.NewNop())
		consumer1 := &mockRunnable{}
		consumer2 := &mockRunnable{}

		group.Add(consumer1)
		group.Add(consumer2)
		_ = group.Start(context.Background())

		err := group.Shutdown()

		require.NoError(t, err)
		assert.True(t, consumer1.shutdown)
		assert.True(t, consumer2.shutdown)
	})

	t.Run("returns first error but shuts down all", func(t *testing.T) {
		sub := newMockSubscriber()
		group := messaging.NewConsumerGroup(sub, zap.NewNop())
		consumer1 := &mockRunnable{shutdownErr: errors.New("shutdown error 1")}
		consumer2 := &mockRunnable{shutdownErr: errors.New("shutdown error 2")}

		group.Add(consumer1)
		group.Add(consumer2)
		_ = group.Start(context.Background())

		err := group.Shutdown()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "shutdown error 1")
		assert.True(t, consumer1.shutdown)
		assert.True(t, consumer2.shutdown) // Still attempted
	})
}
