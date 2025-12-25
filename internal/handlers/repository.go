package handlers

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("url not found")

// URLRepository defines the interface for URL storage operations.
type URLRepository interface {
	Save(ctx context.Context, code, url string) error
	Get(ctx context.Context, code string) (string, error)

	// SaveWithHash saves the code->url mapping and also stores hash->code mapping.
	// Used by hash strategy to enable deduplication.
	SaveWithHash(ctx context.Context, code, url, urlHash string) error

	// GetCodeByHash retrieves the existing code for a given URL hash.
	// Returns ErrNotFound if no mapping exists.
	GetCodeByHash(ctx context.Context, urlHash string) (string, error)
}
