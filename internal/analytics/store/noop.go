package store

import (
	"context"

	"github.com/serroba/web-demo-go/internal/analytics"
	"go.uber.org/zap"
)

// Noop is a no-op implementation of analytics.Store that logs events.
type Noop struct {
	logger *zap.Logger
}

// NewNoop creates a new no-op analytics store.
func NewNoop(logger *zap.Logger) *Noop {
	return &Noop{logger: logger}
}

func (n *Noop) SaveURLCreated(_ context.Context, event *analytics.URLCreatedEvent) error {
	n.logger.Info("url created event received",
		zap.String("code", event.Code),
		zap.String("originalUrl", event.OriginalURL),
		zap.String("strategy", event.Strategy),
		zap.Time("createdAt", event.CreatedAt),
	)

	return nil
}

func (n *Noop) SaveURLAccessed(_ context.Context, event *analytics.URLAccessedEvent) error {
	n.logger.Info("url accessed event received",
		zap.String("code", event.Code),
		zap.Time("accessedAt", event.AccessedAt),
		zap.String("referrer", event.Referrer),
	)

	return nil
}
