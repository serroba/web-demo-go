package messaging

import (
	"context"
	"fmt"

	"github.com/ThreeDotsLabs/watermill/message"
	"go.uber.org/zap"
)

// Runnable represents a component that can be started and shutdown.
type Runnable interface {
	Start(ctx context.Context) error
	Shutdown() error
}

// ConsumerGroup manages multiple consumers with unified lifecycle.
type ConsumerGroup struct {
	consumers  []Runnable
	subscriber message.Subscriber
	logger     *zap.Logger
}

// NewConsumerGroup creates a new consumer group.
func NewConsumerGroup(subscriber message.Subscriber, logger *zap.Logger) *ConsumerGroup {
	return &ConsumerGroup{
		subscriber: subscriber,
		logger:     logger,
	}
}

// Add registers a consumer to the group.
func (g *ConsumerGroup) Add(consumer Runnable) {
	g.consumers = append(g.consumers, consumer)
}

// Start starts all consumers in the group.
func (g *ConsumerGroup) Start(ctx context.Context) error {
	for i, consumer := range g.consumers {
		if err := consumer.Start(ctx); err != nil {
			// Shutdown already started consumers on failure
			for j := i - 1; j >= 0; j-- {
				_ = g.consumers[j].Shutdown()
			}

			return fmt.Errorf("failed to start consumer %d: %w", i, err)
		}
	}

	g.logger.Info("consumer group started", zap.Int("count", len(g.consumers)))

	return nil
}

// Shutdown stops all consumers gracefully.
func (g *ConsumerGroup) Shutdown() error {
	g.logger.Info("shutting down consumer group")

	var firstErr error

	for _, consumer := range g.consumers {
		if err := consumer.Shutdown(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if err := g.subscriber.Close(); err != nil && firstErr == nil {
		firstErr = err
	}

	return firstErr
}
