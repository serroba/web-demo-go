package analytics

import (
	"context"
	"encoding/json"

	"github.com/ThreeDotsLabs/watermill/message"
	"go.uber.org/zap"
)

// Consumer consumes analytics events and persists them to the store.
type Consumer struct {
	subscriber message.Subscriber
	store      Store
	logger     *zap.Logger
	cancel     context.CancelFunc
	done       chan struct{}
}

// NewConsumer creates a new analytics consumer.
func NewConsumer(subscriber message.Subscriber, store Store, logger *zap.Logger) *Consumer {
	return &Consumer{
		subscriber: subscriber,
		store:      store,
		logger:     logger,
		done:       make(chan struct{}),
	}
}

// Start begins consuming messages from both topics.
func (c *Consumer) Start(ctx context.Context) error {
	ctx, c.cancel = context.WithCancel(ctx)

	createdMsgs, err := c.subscriber.Subscribe(ctx, TopicURLCreated)
	if err != nil {
		return err
	}

	accessedMsgs, err := c.subscriber.Subscribe(ctx, TopicURLAccessed)
	if err != nil {
		return err
	}

	go c.consumeLoop(ctx, createdMsgs, accessedMsgs)

	return nil
}

func (c *Consumer) consumeLoop(ctx context.Context, createdMsgs, accessedMsgs <-chan *message.Message) {
	defer close(c.done)

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-createdMsgs:
			if !ok {
				return
			}

			c.handleURLCreated(ctx, msg)
		case msg, ok := <-accessedMsgs:
			if !ok {
				return
			}

			c.handleURLAccessed(ctx, msg)
		}
	}
}

func (c *Consumer) handleURLCreated(ctx context.Context, msg *message.Message) {
	var event URLCreatedEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		c.logger.Error("failed to unmarshal url created event",
			zap.Error(err),
		)
		msg.Nack()

		return
	}

	if err := c.store.SaveURLCreated(ctx, &event); err != nil {
		c.logger.Error("failed to save url created event",
			zap.String("code", event.Code),
			zap.Error(err),
		)
		msg.Nack()

		return
	}

	msg.Ack()

	c.logger.Debug("processed url created event",
		zap.String("code", event.Code),
	)
}

func (c *Consumer) handleURLAccessed(ctx context.Context, msg *message.Message) {
	var event URLAccessedEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		c.logger.Error("failed to unmarshal url accessed event",
			zap.Error(err),
		)
		msg.Nack()

		return
	}

	if err := c.store.SaveURLAccessed(ctx, &event); err != nil {
		c.logger.Error("failed to save url accessed event",
			zap.String("code", event.Code),
			zap.Error(err),
		)
		msg.Nack()

		return
	}

	msg.Ack()

	c.logger.Debug("processed url accessed event",
		zap.String("code", event.Code),
	)
}

// Shutdown stops the consumer and waits for in-flight messages to complete.
func (c *Consumer) Shutdown() error {
	if c.cancel != nil {
		c.cancel()
	}

	<-c.done

	return c.subscriber.Close()
}
