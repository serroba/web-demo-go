package messaging_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"github.com/serroba/web-demo-go/internal/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type testEvent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type mockSubscriber struct {
	msgChan      chan *message.Message
	subscribeErr error
	mu           sync.Mutex
	closed       bool
}

func newMockSubscriber() *mockSubscriber {
	return &mockSubscriber{
		msgChan: make(chan *message.Message, 10),
	}
}

func (m *mockSubscriber) Subscribe(_ context.Context, _ string) (<-chan *message.Message, error) {
	if m.subscribeErr != nil {
		return nil, m.subscribeErr
	}

	return m.msgChan, nil
}

func (m *mockSubscriber) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.closed {
		m.closed = true
		close(m.msgChan)
	}

	return nil
}

func TestConsumer_Start(t *testing.T) {
	t.Run("starts successfully", func(t *testing.T) {
		sub := newMockSubscriber()
		consumer := messaging.NewConsumer(
			sub,
			"test.topic",
			func(_ context.Context, _ *testEvent) error { return nil },
			zap.NewNop(),
		)

		err := consumer.Start(context.Background())

		require.NoError(t, err)
		assert.Equal(t, "test.topic", consumer.Topic())

		_ = consumer.Shutdown()
	})

	t.Run("returns error when subscribe fails", func(t *testing.T) {
		sub := &mockSubscriber{subscribeErr: errors.New("subscribe error")}
		consumer := messaging.NewConsumer(
			sub,
			"test.topic",
			func(_ context.Context, _ *testEvent) error { return nil },
			zap.NewNop(),
		)

		err := consumer.Start(context.Background())

		assert.Error(t, err)
	})
}

func TestConsumer_HandleMessage(t *testing.T) {
	t.Run("acks on successful handling", func(t *testing.T) {
		sub := newMockSubscriber()

		var receivedEvent *testEvent

		consumer := messaging.NewConsumer(
			sub,
			"test.topic",
			func(_ context.Context, event *testEvent) error {
				receivedEvent = event

				return nil
			},
			zap.NewNop(),
		)

		err := consumer.Start(context.Background())
		require.NoError(t, err)

		event := &testEvent{ID: "123", Name: "test"}
		payload, _ := json.Marshal(event)
		msg := message.NewMessage(uuid.NewString(), payload)

		sub.msgChan <- msg

		select {
		case <-msg.Acked():
			assert.Equal(t, "123", receivedEvent.ID)
			assert.Equal(t, "test", receivedEvent.Name)
		case <-msg.Nacked():
			t.Fatal("message was nacked")
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for ack")
		}

		_ = consumer.Shutdown()
	})

	t.Run("nacks on unmarshal error", func(t *testing.T) {
		sub := newMockSubscriber()
		consumer := messaging.NewConsumer(
			sub,
			"test.topic",
			func(_ context.Context, _ *testEvent) error { return nil },
			zap.NewNop(),
		)

		err := consumer.Start(context.Background())
		require.NoError(t, err)

		msg := message.NewMessage(uuid.NewString(), []byte("invalid json"))

		sub.msgChan <- msg

		select {
		case <-msg.Nacked():
			// Success
		case <-msg.Acked():
			t.Fatal("message should have been nacked")
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for nack")
		}

		_ = consumer.Shutdown()
	})

	t.Run("nacks on handler error", func(t *testing.T) {
		sub := newMockSubscriber()
		consumer := messaging.NewConsumer(
			sub,
			"test.topic",
			func(_ context.Context, _ *testEvent) error {
				return errors.New("handler error")
			},
			zap.NewNop(),
		)

		err := consumer.Start(context.Background())
		require.NoError(t, err)

		event := &testEvent{ID: "123"}
		payload, _ := json.Marshal(event)
		msg := message.NewMessage(uuid.NewString(), payload)

		sub.msgChan <- msg

		select {
		case <-msg.Nacked():
			// Success
		case <-msg.Acked():
			t.Fatal("message should have been nacked")
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for nack")
		}

		_ = consumer.Shutdown()
	})
}

func TestConsumer_Shutdown(t *testing.T) {
	t.Run("shuts down gracefully", func(t *testing.T) {
		sub := newMockSubscriber()
		consumer := messaging.NewConsumer(
			sub,
			"test.topic",
			func(_ context.Context, _ *testEvent) error { return nil },
			zap.NewNop(),
		)

		err := consumer.Start(context.Background())
		require.NoError(t, err)

		err = consumer.Shutdown()

		require.NoError(t, err)
	})
}
