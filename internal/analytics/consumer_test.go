package analytics_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"github.com/serroba/web-demo-go/internal/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type mockSubscriber struct {
	createdChan  chan *message.Message
	accessedChan chan *message.Message
	subscribeErr error
	closeErr     error
	mu           sync.Mutex
	closed       bool
}

func newMockSubscriber() *mockSubscriber {
	return &mockSubscriber{
		createdChan:  make(chan *message.Message, 10),
		accessedChan: make(chan *message.Message, 10),
	}
}

func (m *mockSubscriber) Subscribe(_ context.Context, topic string) (<-chan *message.Message, error) {
	if m.subscribeErr != nil {
		return nil, m.subscribeErr
	}

	switch topic {
	case analytics.TopicURLCreated:
		return m.createdChan, nil
	case analytics.TopicURLAccessed:
		return m.accessedChan, nil
	default:
		return nil, errors.New("unknown topic")
	}
}

func (m *mockSubscriber) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.closed {
		m.closed = true
		close(m.createdChan)
		close(m.accessedChan)
	}

	return m.closeErr
}

type mockStore struct {
	createdEvents   []*analytics.URLCreatedEvent
	accessedEvents  []*analytics.URLAccessedEvent
	saveCreatedErr  error
	saveAccessedErr error
	mu              sync.Mutex
}

func (m *mockStore) SaveURLCreated(_ context.Context, event *analytics.URLCreatedEvent) error {
	if m.saveCreatedErr != nil {
		return m.saveCreatedErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.createdEvents = append(m.createdEvents, event)

	return nil
}

func (m *mockStore) SaveURLAccessed(_ context.Context, event *analytics.URLAccessedEvent) error {
	if m.saveAccessedErr != nil {
		return m.saveAccessedErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.accessedEvents = append(m.accessedEvents, event)

	return nil
}

func TestNewConsumer(t *testing.T) {
	sub := newMockSubscriber()
	store := &mockStore{}
	logger := zap.NewNop()

	consumer := analytics.NewConsumer(sub, store, logger)

	assert.NotNil(t, consumer)
}

func TestConsumer_Start(t *testing.T) {
	t.Run("starts successfully", func(t *testing.T) {
		sub := newMockSubscriber()
		store := &mockStore{}
		logger := zap.NewNop()
		consumer := analytics.NewConsumer(sub, store, logger)

		err := consumer.Start(context.Background())

		require.NoError(t, err)

		_ = consumer.Shutdown()
	})

	t.Run("returns error when first subscription fails", func(t *testing.T) {
		sub := &mockSubscriber{subscribeErr: errors.New("subscribe error")}
		store := &mockStore{}
		logger := zap.NewNop()
		consumer := analytics.NewConsumer(sub, store, logger)

		err := consumer.Start(context.Background())

		assert.Error(t, err)
	})
}

func TestConsumer_ProcessURLCreated(t *testing.T) {
	t.Run("processes url created event successfully", func(t *testing.T) {
		sub := newMockSubscriber()
		store := &mockStore{}
		logger := zap.NewNop()
		consumer := analytics.NewConsumer(sub, store, logger)

		err := consumer.Start(context.Background())
		require.NoError(t, err)

		event := &analytics.URLCreatedEvent{
			Code:        "abc123",
			OriginalURL: "https://example.com",
			Strategy:    "token",
			CreatedAt:   time.Now(),
		}

		payload, _ := json.Marshal(event)
		msg := message.NewMessage(uuid.NewString(), payload)

		sub.createdChan <- msg

		// Wait for message to be processed
		select {
		case <-msg.Acked():
			// Success
		case <-msg.Nacked():
			t.Fatal("message was nacked")
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for ack")
		}

		store.mu.Lock()
		defer store.mu.Unlock()

		assert.Len(t, store.createdEvents, 1)
		assert.Equal(t, "abc123", store.createdEvents[0].Code)

		_ = consumer.Shutdown()
	})

	t.Run("nacks on unmarshal error", func(t *testing.T) {
		sub := newMockSubscriber()
		store := &mockStore{}
		logger := zap.NewNop()
		consumer := analytics.NewConsumer(sub, store, logger)

		err := consumer.Start(context.Background())
		require.NoError(t, err)

		msg := message.NewMessage(uuid.NewString(), []byte("invalid json"))

		sub.createdChan <- msg

		select {
		case <-msg.Nacked():
			// Success - message was nacked
		case <-msg.Acked():
			t.Fatal("message should have been nacked")
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for nack")
		}

		_ = consumer.Shutdown()
	})

	t.Run("nacks on store error", func(t *testing.T) {
		sub := newMockSubscriber()
		store := &mockStore{saveCreatedErr: errors.New("store error")}
		logger := zap.NewNop()
		consumer := analytics.NewConsumer(sub, store, logger)

		err := consumer.Start(context.Background())
		require.NoError(t, err)

		event := &analytics.URLCreatedEvent{Code: "abc123"}
		payload, _ := json.Marshal(event)
		msg := message.NewMessage(uuid.NewString(), payload)

		sub.createdChan <- msg

		select {
		case <-msg.Nacked():
			// Success - message was nacked
		case <-msg.Acked():
			t.Fatal("message should have been nacked")
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for nack")
		}

		_ = consumer.Shutdown()
	})
}

func TestConsumer_ProcessURLAccessed(t *testing.T) {
	t.Run("processes url accessed event successfully", func(t *testing.T) {
		sub := newMockSubscriber()
		store := &mockStore{}
		logger := zap.NewNop()
		consumer := analytics.NewConsumer(sub, store, logger)

		err := consumer.Start(context.Background())
		require.NoError(t, err)

		event := &analytics.URLAccessedEvent{
			Code:       "abc123",
			AccessedAt: time.Now(),
			ClientIP:   "127.0.0.1",
		}

		payload, _ := json.Marshal(event)
		msg := message.NewMessage(uuid.NewString(), payload)

		sub.accessedChan <- msg

		select {
		case <-msg.Acked():
			// Success
		case <-msg.Nacked():
			t.Fatal("message was nacked")
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for ack")
		}

		store.mu.Lock()
		defer store.mu.Unlock()

		assert.Len(t, store.accessedEvents, 1)
		assert.Equal(t, "abc123", store.accessedEvents[0].Code)

		_ = consumer.Shutdown()
	})

	t.Run("nacks on unmarshal error", func(t *testing.T) {
		sub := newMockSubscriber()
		store := &mockStore{}
		logger := zap.NewNop()
		consumer := analytics.NewConsumer(sub, store, logger)

		err := consumer.Start(context.Background())
		require.NoError(t, err)

		msg := message.NewMessage(uuid.NewString(), []byte("invalid json"))

		sub.accessedChan <- msg

		select {
		case <-msg.Nacked():
			// Success - message was nacked
		case <-msg.Acked():
			t.Fatal("message should have been nacked")
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for nack")
		}

		_ = consumer.Shutdown()
	})

	t.Run("nacks on store error", func(t *testing.T) {
		sub := newMockSubscriber()
		store := &mockStore{saveAccessedErr: errors.New("store error")}
		logger := zap.NewNop()
		consumer := analytics.NewConsumer(sub, store, logger)

		err := consumer.Start(context.Background())
		require.NoError(t, err)

		event := &analytics.URLAccessedEvent{Code: "abc123"}
		payload, _ := json.Marshal(event)
		msg := message.NewMessage(uuid.NewString(), payload)

		sub.accessedChan <- msg

		select {
		case <-msg.Nacked():
			// Success - message was nacked
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
		store := &mockStore{}
		logger := zap.NewNop()
		consumer := analytics.NewConsumer(sub, store, logger)

		err := consumer.Start(context.Background())
		require.NoError(t, err)

		err = consumer.Shutdown()

		require.NoError(t, err)
	})

	t.Run("returns error when close fails", func(t *testing.T) {
		sub := newMockSubscriber()
		sub.closeErr = errors.New("close error")
		store := &mockStore{}
		logger := zap.NewNop()
		consumer := analytics.NewConsumer(sub, store, logger)

		err := consumer.Start(context.Background())
		require.NoError(t, err)

		err = consumer.Shutdown()

		assert.Error(t, err)
	})
}
