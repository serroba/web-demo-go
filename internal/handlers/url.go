package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/serroba/web-demo-go/internal/analytics"
	"github.com/serroba/web-demo-go/internal/shortener"
	"go.uber.org/zap"
)

// URLHandler handles URL shortening operations.
type URLHandler struct {
	strategies      map[Strategy]shortener.Strategy
	store           shortener.Repository
	baseURL         string
	defaultStrategy Strategy
	publisher       *analytics.Publisher
	logger          *zap.Logger
}

// NewURLHandler creates a new URL handler with injected strategies.
func NewURLHandler(
	store shortener.Repository,
	baseURL string,
	strategies map[Strategy]shortener.Strategy,
	publisher *analytics.Publisher,
	logger *zap.Logger,
) *URLHandler {
	return &URLHandler{
		strategies:      strategies,
		store:           store,
		baseURL:         baseURL,
		defaultStrategy: StrategyToken,
		publisher:       publisher,
		logger:          logger,
	}
}

type contextKey string

const (
	clientIPKey  contextKey = "clientIP"
	userAgentKey contextKey = "userAgent"
)

// ContextWithRequestMeta adds client IP and user-agent to context.
func ContextWithRequestMeta(ctx context.Context, clientIP, userAgent string) context.Context {
	ctx = context.WithValue(ctx, clientIPKey, clientIP)
	ctx = context.WithValue(ctx, userAgentKey, userAgent)

	return ctx
}

func clientIPFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(clientIPKey).(string); ok {
		return v
	}

	return ""
}

func userAgentFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(userAgentKey).(string); ok {
		return v
	}

	return ""
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
	event := &analytics.URLCreatedEvent{
		Code:        string(shortURL.Code),
		OriginalURL: shortURL.OriginalURL,
		URLHash:     string(shortURL.URLHash),
		Strategy:    string(strategyName),
		CreatedAt:   shortURL.CreatedAt,
		ClientIP:    clientIPFromContext(ctx),
		UserAgent:   userAgentFromContext(ctx),
	}

	if err := h.publisher.PublishURLCreated(event); err != nil {
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

	resp := &RedirectResponse{
		Status: http.StatusMovedPermanently,
	}
	resp.Headers.Location = shortURL.OriginalURL

	return resp, nil
}
