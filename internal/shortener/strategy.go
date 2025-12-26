package shortener

import (
	"context"
	"errors"
	"time"
)

// Strategy defines the interface for URL shortening strategies.
type Strategy interface {
	Shorten(ctx context.Context, url string) (*ShortURL, error)
}

// CodeGenerator generates unique short codes.
type CodeGenerator func() string

// TokenStrategy always generates a new code for each URL.
type TokenStrategy struct {
	store        Repository
	generateCode CodeGenerator
}

// NewTokenStrategy creates a new token-based shortening strategy.
func NewTokenStrategy(store Repository, generator CodeGenerator) *TokenStrategy {
	return &TokenStrategy{
		store:        store,
		generateCode: generator,
	}
}

func (s *TokenStrategy) Shorten(ctx context.Context, url string) (*ShortURL, error) {
	shortURL := &ShortURL{
		Code:        Code(s.generateCode()),
		OriginalURL: url,
		URLHash:     "",
		CreatedAt:   time.Now(),
	}

	if err := s.store.Save(ctx, shortURL); err != nil {
		return nil, err
	}

	return shortURL, nil
}

// HashStrategy deduplicates URLs by returning the same code for identical URLs.
type HashStrategy struct {
	store        Repository
	generateCode CodeGenerator
}

// NewHashStrategy creates a new hash-based shortening strategy.
func NewHashStrategy(store Repository, generator CodeGenerator) *HashStrategy {
	return &HashStrategy{
		store:        store,
		generateCode: generator,
	}
}

func (s *HashStrategy) Shorten(ctx context.Context, rawURL string) (*ShortURL, error) {
	normalizedURL, err := NormalizeURL(rawURL)
	if err != nil {
		return nil, err
	}

	urlHash := URLHash(HashURL(normalizedURL))

	existing, err := s.store.GetByHash(ctx, urlHash)
	if err == nil {
		return existing, nil
	}

	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	shortURL := &ShortURL{
		Code:        Code(s.generateCode()),
		OriginalURL: rawURL,
		URLHash:     urlHash,
		CreatedAt:   time.Now(),
	}

	if err = s.store.Save(ctx, shortURL); err != nil {
		return nil, err
	}

	return shortURL, nil
}
