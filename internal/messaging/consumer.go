package messaging

import (
	"context"
	"encoding/json"

	"github.com/ThreeDotsLabs/watermill/message"
	"go.uber.org/zap"
)

// Handler processes a single event. Handlers are synchronous and easy to test.
type Handler[T any] func(ctx context.Context, event *T) error

// Consumer subscribes to a topic and processes messages with a typed handler.
type Consumer[T any] struct {
	subscriber message.Subscriber
	topic      string
	handler    Handler[T]
	logger     *zap.Logger
	cancel     context.CancelFunc
	done       chan struct{}
}

// NewConsumer creates a new generic consumer for a specific event type.
func NewConsumer[T any](
	subscriber message.Subscriber,
	topic string,
	handler Handler[T],
	logger *zap.Logger,
) *Consumer[T] {
	return &Consumer[T]{
		subscriber: subscriber,
		topic:      topic,
		handler:    handler,
		logger:     logger,
		done:       make(chan struct{}),
	}
}

// Topic returns the topic this consumer subscribes to.
func (c *Consumer[T]) Topic() string {
	return c.topic
}

// Start begins consuming messages from the topic.
func (c *Consumer[T]) Start(ctx context.Context) error {
	ctx, c.cancel = context.WithCancel(ctx)

	msgs, err := c.subscriber.Subscribe(ctx, c.topic)
	if err != nil {
		return err
	}

	go c.consumeLoop(ctx, msgs)

	return nil
}

func (c *Consumer[T]) consumeLoop(ctx context.Context, msgs <-chan *message.Message) {
	defer close(c.done)

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-msgs:
			if !ok {
				return
			}

			c.handleMessage(ctx, msg)
		}
	}
}

func (c *Consumer[T]) handleMessage(ctx context.Context, msg *message.Message) {
	var event T
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		c.logger.Error("failed to unmarshal event",
			zap.String("topic", c.topic),
			zap.Error(err),
		)
		msg.Nack()

		return
	}

	if err := c.handler(ctx, &event); err != nil {
		c.logger.Error("failed to handle event",
			zap.String("topic", c.topic),
			zap.Error(err),
		)
		msg.Nack()

		return
	}

	msg.Ack()

	c.logger.Debug("processed event",
		zap.String("topic", c.topic),
	)
}

// Shutdown stops the consumer and waits for in-flight messages to complete.
func (c *Consumer[T]) Shutdown() error {
	if c.cancel != nil {
		c.cancel()
	}

	<-c.done

	return nil
}
