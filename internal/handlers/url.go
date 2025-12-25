package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/serroba/web-demo-go/internal/shortener"
)

// URLHandler handles URL shortening operations.
type URLHandler struct {
	strategies      map[Strategy]shortener.Strategy
	store           shortener.Repository
	baseURL         string
	defaultStrategy Strategy
}

// NewURLHandler creates a new URL handler with injected strategies.
func NewURLHandler(
	store shortener.Repository,
	baseURL string,
	strategies map[Strategy]shortener.Strategy,
) *URLHandler {
	return &URLHandler{
		strategies:      strategies,
		store:           store,
		baseURL:         baseURL,
		defaultStrategy: StrategyToken,
	}
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
