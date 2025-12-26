package analytics

import (
	"encoding/json"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
)

const TopicURLCreated = "url.created"

// Publisher publishes analytics events.
type Publisher struct {
	publisher message.Publisher
}

// NewPublisher creates a new analytics publisher.
func NewPublisher(publisher message.Publisher) *Publisher {
	return &Publisher{publisher: publisher}
}

// PublishURLCreated publishes a URL created event.
func (p *Publisher) PublishURLCreated(event *URLCreatedEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := message.NewMessage(watermill.NewUUID(), payload)

	return p.publisher.Publish(TopicURLCreated, msg)
}

// Shutdown closes the underlying publisher.
func (p *Publisher) Shutdown() error {
	return p.publisher.Close()
}
