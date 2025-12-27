package messaging

import (
	"encoding/json"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
)

// Publish is a function that publishes a typed event.
type Publish[T any] func(event *T) error

// NewPublishFunc creates a typed publish function for a specific topic.
func NewPublishFunc[T any](publisher message.Publisher, topic string) Publish[T] {
	return func(event *T) error {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}

		msg := message.NewMessage(watermill.NewUUID(), payload)

		return publisher.Publish(topic, msg)
	}
}

// PublisherGroup manages the underlying publisher lifecycle.
type PublisherGroup struct {
	publisher message.Publisher
}

// NewPublisherGroup creates a new publisher group.
func NewPublisherGroup(publisher message.Publisher) *PublisherGroup {
	return &PublisherGroup{publisher: publisher}
}

// Publisher returns the underlying message publisher for creating typed publish functions.
func (g *PublisherGroup) Publisher() message.Publisher {
	return g.publisher
}

// Shutdown closes the underlying publisher.
func (g *PublisherGroup) Shutdown() error {
	return g.publisher.Close()
}
