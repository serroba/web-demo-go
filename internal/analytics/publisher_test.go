package analytics_test

import (
	"errors"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/serroba/web-demo-go/internal/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPublisher struct {
	messages   []*message.Message
	topic      string
	publishErr error
	closeErr   error
}

func (m *mockPublisher) Publish(topic string, msgs ...*message.Message) error {
	if m.publishErr != nil {
		return m.publishErr
	}

	m.topic = topic
	m.messages = append(m.messages, msgs...)

	return nil
}

func (m *mockPublisher) Close() error {
	return m.closeErr
}

func TestPublisher_PublishURLCreated(t *testing.T) {
	t.Run("publishes event successfully", func(t *testing.T) {
		mock := &mockPublisher{}
		pub := analytics.NewPublisher(mock)

		event := &analytics.URLCreatedEvent{
			Code:        "abc123",
			OriginalURL: "https://example.com",
			Strategy:    "token",
			CreatedAt:   time.Now(),
		}

		err := pub.PublishURLCreated(event)

		require.NoError(t, err)
		assert.Equal(t, analytics.TopicURLCreated, mock.topic)
		assert.Len(t, mock.messages, 1)
	})

	t.Run("returns error when publish fails", func(t *testing.T) {
		mock := &mockPublisher{publishErr: errors.New("publish error")}
		pub := analytics.NewPublisher(mock)

		event := &analytics.URLCreatedEvent{Code: "abc123"}

		err := pub.PublishURLCreated(event)

		assert.Error(t, err)
	})
}

func TestPublisher_PublishURLAccessed(t *testing.T) {
	t.Run("publishes event successfully", func(t *testing.T) {
		mock := &mockPublisher{}
		pub := analytics.NewPublisher(mock)

		event := &analytics.URLAccessedEvent{
			Code:       "abc123",
			AccessedAt: time.Now(),
			ClientIP:   "127.0.0.1",
		}

		err := pub.PublishURLAccessed(event)

		require.NoError(t, err)
		assert.Equal(t, analytics.TopicURLAccessed, mock.topic)
		assert.Len(t, mock.messages, 1)
	})

	t.Run("returns error when publish fails", func(t *testing.T) {
		mock := &mockPublisher{publishErr: errors.New("publish error")}
		pub := analytics.NewPublisher(mock)

		event := &analytics.URLAccessedEvent{Code: "abc123"}

		err := pub.PublishURLAccessed(event)

		assert.Error(t, err)
	})
}

func TestPublisher_Shutdown(t *testing.T) {
	t.Run("closes underlying publisher", func(t *testing.T) {
		mock := &mockPublisher{}
		pub := analytics.NewPublisher(mock)

		err := pub.Shutdown()

		require.NoError(t, err)
	})

	t.Run("returns error when close fails", func(t *testing.T) {
		mock := &mockPublisher{closeErr: errors.New("close error")}
		pub := analytics.NewPublisher(mock)

		err := pub.Shutdown()

		assert.Error(t, err)
	})
}
