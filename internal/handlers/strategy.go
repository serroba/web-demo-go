package handlers

import (
	"context"
	"errors"

	"github.com/serroba/web-demo-go/internal/domain"
)

// ShortenerStrategy defines the interface for URL shortening strategies.
type ShortenerStrategy interface {
	Shorten(ctx context.Context, url string) (*domain.ShortURL, error)
}

// CodeGenerator generates unique short codes.
type CodeGenerator func() string

// TokenStrategy always generates a new code for each URL.
type TokenStrategy struct {
	store        domain.ShortURLRepository
	generateCode CodeGenerator
}

// NewTokenStrategy creates a new token-based shortening strategy.
func NewTokenStrategy(store domain.ShortURLRepository, generator CodeGenerator) *TokenStrategy {
	return &TokenStrategy{
		store:        store,
		generateCode: generator,
	}
}

func (s *TokenStrategy) Shorten(ctx context.Context, url string) (*domain.ShortURL, error) {
	shortURL := &domain.ShortURL{
		Code:        domain.Code(s.generateCode()),
		OriginalURL: url,
		URLHash:     "",
	}

	if err := s.store.Save(ctx, shortURL); err != nil {
		return nil, err
	}

	return shortURL, nil
}

// HashStrategy deduplicates URLs by returning the same code for identical URLs.
type HashStrategy struct {
	store        domain.ShortURLRepository
	generateCode CodeGenerator
}

// NewHashStrategy creates a new hash-based shortening strategy.
func NewHashStrategy(store domain.ShortURLRepository, generator CodeGenerator) *HashStrategy {
	return &HashStrategy{
		store:        store,
		generateCode: generator,
	}
}

func (s *HashStrategy) Shorten(ctx context.Context, rawURL string) (*domain.ShortURL, error) {
	normalizedURL, err := NormalizeURL(rawURL)
	if err != nil {
		return nil, err
	}

	urlHash := domain.URLHash(HashURL(normalizedURL))

	existing, err := s.store.GetByHash(ctx, urlHash)
	if err == nil {
		return existing, nil
	}

	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	shortURL := &domain.ShortURL{
		Code:        domain.Code(s.generateCode()),
		OriginalURL: rawURL,
		URLHash:     urlHash,
	}

	if err = s.store.Save(ctx, shortURL); err != nil {
		return nil, err
	}

	return shortURL, nil
}
