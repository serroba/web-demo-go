package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jaevor/go-nanoid"
)

// URLHandler handles URL shortening operations.
type URLHandler struct {
	store      URLRepository
	generateID func() string
	baseURL    string
}

// NewURLHandler creates a new URL handler with the given store.
func NewURLHandler(s URLRepository, baseURL string, codeLength int) *URLHandler {
	gen, _ := nanoid.Standard(codeLength)

	return &URLHandler{
		store:      s,
		generateID: gen,
		baseURL:    baseURL,
	}
}

func (h *URLHandler) CreateShortURL(ctx context.Context, req *CreateShortURLRequest) (*CreateShortURLResponse, error) {
	if req.Body.URL == "" {
		return nil, huma.Error400BadRequest("url is required")
	}

	// Determine strategy (default to token)
	strategy := req.Body.Strategy
	if strategy == "" {
		strategy = StrategyToken
	}

	var code string

	var err error

	switch strategy {
	case StrategyHash:
		code, err = h.createWithHashStrategy(ctx, req.Body.URL)
	case StrategyToken:
		code, err = h.createWithTokenStrategy(ctx, req.Body.URL)
	default:
		return nil, huma.Error400BadRequest("invalid strategy: must be 'token' or 'hash'")
	}

	if err != nil {
		return nil, huma.Error500InternalServerError("failed to save url")
	}

	shortURL := fmt.Sprintf("%s/%s", h.baseURL, code)

	resp := &CreateShortURLResponse{}
	resp.Headers.Location = shortURL
	resp.Body.Code = code
	resp.Body.ShortURL = shortURL
	resp.Body.OriginalURL = req.Body.URL

	return resp, nil
}

// createWithTokenStrategy always generates a new code (current behavior).
func (h *URLHandler) createWithTokenStrategy(ctx context.Context, url string) (string, error) {
	code := h.generateID()
	if err := h.store.Save(ctx, code, url); err != nil {
		return "", err
	}

	return code, nil
}

// createWithHashStrategy checks for existing hash mapping first (deduplication).
func (h *URLHandler) createWithHashStrategy(ctx context.Context, rawURL string) (string, error) {
	// Normalize URL for consistent hashing
	normalizedURL, err := NormalizeURL(rawURL)
	if err != nil {
		return "", err
	}

	// Compute hash of normalized URL
	urlHash := HashURL(normalizedURL)

	// Check if we already have a code for this hash
	existingCode, err := h.store.GetCodeByHash(ctx, urlHash)
	if err == nil {
		// Found existing code - return it (deduplication)
		return existingCode, nil
	}

	if !errors.Is(err, ErrNotFound) {
		// Unexpected error
		return "", err
	}

	// No existing mapping - generate new code and save both mappings
	code := h.generateID()
	if err = h.store.SaveWithHash(ctx, code, rawURL, urlHash); err != nil {
		return "", err
	}

	return code, nil
}

func (h *URLHandler) RedirectToURL(ctx context.Context, req *RedirectRequest) (*RedirectResponse, error) {
	url, err := h.store.Get(ctx, req.Code)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, huma.Error404NotFound("short url not found")
		}

		return nil, huma.Error500InternalServerError("failed to get url")
	}

	resp := &RedirectResponse{
		Status: http.StatusMovedPermanently,
	}
	resp.Headers.Location = url

	return resp, nil
}
