package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/serroba/web-demo-go/internal/analytics"
	"github.com/serroba/web-demo-go/internal/messaging"
	"github.com/serroba/web-demo-go/internal/shortener"
	"go.uber.org/zap"
)

// URLHandler handles URL shortening operations.
type URLHandler struct {
	strategies         map[Strategy]shortener.Strategy
	store              shortener.Repository
	baseURL            string
	defaultStrategy    Strategy
	publishURLCreated  messaging.Publish[analytics.URLCreatedEvent]
	publishURLAccessed messaging.Publish[analytics.URLAccessedEvent]
	logger             *zap.Logger
}

// NewURLHandler creates a new URL handler with injected strategies.
func NewURLHandler(
	store shortener.Repository,
	baseURL string,
	strategies map[Strategy]shortener.Strategy,
	publishURLCreated messaging.Publish[analytics.URLCreatedEvent],
	publishURLAccessed messaging.Publish[analytics.URLAccessedEvent],
	logger *zap.Logger,
) *URLHandler {
	return &URLHandler{
		strategies:         strategies,
		store:              store,
		baseURL:            baseURL,
		defaultStrategy:    StrategyToken,
		publishURLCreated:  publishURLCreated,
		publishURLAccessed: publishURLAccessed,
		logger:             logger,
	}
}

type requestMetaKey struct{}

// RequestMeta holds HTTP request metadata for analytics.
type RequestMeta struct {
	ClientIP  string
	UserAgent string
	Referrer  string
}

// ContextWithRequestMeta adds request metadata to context.
func ContextWithRequestMeta(ctx context.Context, meta RequestMeta) context.Context {
	return context.WithValue(ctx, requestMetaKey{}, meta)
}

// RequestMetaFromContext extracts request metadata from context.
func RequestMetaFromContext(ctx context.Context) RequestMeta {
	if v, ok := ctx.Value(requestMetaKey{}).(RequestMeta); ok {
		return v
	}

	return RequestMeta{}
}

func (h *URLHandler) CreateShortURL(ctx context.Context, req *CreateShortURLRequest) (*CreateShortURLResponse, error) {
	strategyName := req.Body.Strategy
	if strategyName == "" {
		strategyName = h.defaultStrategy
	}

	strategy, ok := h.strategies[strategyName]
	if !ok {
		return nil, huma.Error400BadRequest("invalid strategy: must be 'token' or 'hash'")
	}

	shortURL, err := strategy.Shorten(ctx, req.Body.URL)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to save url")
	}

	// Publish analytics event
	meta := RequestMetaFromContext(ctx)
	event := &analytics.URLCreatedEvent{
		Code:        string(shortURL.Code),
		OriginalURL: shortURL.OriginalURL,
		URLHash:     string(shortURL.URLHash),
		Strategy:    string(strategyName),
		CreatedAt:   shortURL.CreatedAt,
		ClientIP:    meta.ClientIP,
		UserAgent:   meta.UserAgent,
	}

	if err := h.publishURLCreated(event); err != nil {
		h.logger.Error("failed to publish analytics event",
			zap.String("code", event.Code),
			zap.Error(err),
		)
	}

	fullShortURL := fmt.Sprintf("%s/%s", h.baseURL, shortURL.Code)

	resp := &CreateShortURLResponse{}
	resp.Headers.Location = fullShortURL
	resp.Body.Code = string(shortURL.Code)
	resp.Body.ShortURL = fullShortURL
	resp.Body.OriginalURL = shortURL.OriginalURL

	return resp, nil
}

func (h *URLHandler) RedirectToURL(ctx context.Context, req *RedirectRequest) (*RedirectResponse, error) {
	shortURL, err := h.store.GetByCode(ctx, shortener.Code(req.Code))
	if err != nil {
		if errors.Is(err, shortener.ErrNotFound) {
			return nil, huma.Error404NotFound("short url not found")
		}

		return nil, huma.Error500InternalServerError("failed to get url")
	}

	meta := RequestMetaFromContext(ctx)
	event := &analytics.URLAccessedEvent{
		Code:       req.Code,
		AccessedAt: time.Now(),
		ClientIP:   meta.ClientIP,
		UserAgent:  meta.UserAgent,
		Referrer:   meta.Referrer,
	}

	if err = h.publishURLAccessed(event); err != nil {
		h.logger.Error("failed to publish access event",
			zap.String("code", event.Code),
			zap.Error(err),
		)
	}

	resp := &RedirectResponse{
		Status: http.StatusMovedPermanently,
	}
	resp.Headers.Location = shortURL.OriginalURL

	return resp, nil
}
