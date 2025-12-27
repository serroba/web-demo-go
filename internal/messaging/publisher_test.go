package messaging_test

import (
	"errors"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/serroba/web-demo-go/internal/messaging"
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

type publishTestEvent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func TestNewPublishFunc(t *testing.T) {
	t.Run("publishes event successfully", func(t *testing.T) {
		mock := &mockPublisher{}
		publish := messaging.NewPublishFunc[publishTestEvent](mock, "test.topic")

		event := &publishTestEvent{ID: "123", Name: "test"}

		err := publish(event)

		require.NoError(t, err)
		assert.Equal(t, "test.topic", mock.topic)
		assert.Len(t, mock.messages, 1)
		assert.Contains(t, string(mock.messages[0].Payload), `"id":"123"`)
	})

	t.Run("returns error when publish fails", func(t *testing.T) {
		mock := &mockPublisher{publishErr: errors.New("publish error")}
		publish := messaging.NewPublishFunc[publishTestEvent](mock, "test.topic")

		event := &publishTestEvent{ID: "123"}

		err := publish(event)

		assert.Error(t, err)
	})
}

func TestPublisherGroup(t *testing.T) {
	t.Run("returns underlying publisher", func(t *testing.T) {
		mock := &mockPublisher{}
		group := messaging.NewPublisherGroup(mock)

		assert.Equal(t, mock, group.Publisher())
	})

	t.Run("shuts down successfully", func(t *testing.T) {
		mock := &mockPublisher{}
		group := messaging.NewPublisherGroup(mock)

		err := group.Shutdown()

		require.NoError(t, err)
	})

	t.Run("returns error when close fails", func(t *testing.T) {
		mock := &mockPublisher{closeErr: errors.New("close error")}
		group := messaging.NewPublisherGroup(mock)

		err := group.Shutdown()

		assert.Error(t, err)
	})
}
