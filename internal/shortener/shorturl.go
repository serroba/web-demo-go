package shortener

import "time"

// Code represents a short URL code.
type Code string

// URLHash represents a hash of a normalized URL.
type URLHash string

// ShortURL represents a shortened URL entity.
type ShortURL struct {
	Code        Code
	OriginalURL string
	URLHash     URLHash // empty for token strategy, populated for hash strategy
	CreatedAt   time.Time
}
