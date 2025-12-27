package store

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/serroba/web-demo-go/internal/shortener"
)

// RedisCacheRepository wraps a Repository with Redis caching for reads.
type RedisCacheRepository struct {
	store   shortener.Repository
	client  *redis.Client
	prefix  string
	hashKey string
	ttl     time.Duration
}

// NewRedisCacheRepository creates a new Redis-cached repository decorator.
func NewRedisCacheRepository(
	store shortener.Repository, client *redis.Client, ttl time.Duration,
) *RedisCacheRepository {
	return &RedisCacheRepository{
		store:   store,
		client:  client,
		prefix:  "url:",
		hashKey: "url_hashes",
		ttl:     ttl,
	}
}

// Save stores a short URL in the underlying store and updates the cache.
func (r *RedisCacheRepository) Save(ctx context.Context, shortURL *shortener.ShortURL) error {
	if err := r.store.Save(ctx, shortURL); err != nil {
		return err
	}

	// Write-through: update cache after successful save
	r.cacheURL(ctx, shortURL)

	return nil
}

// GetByCode retrieves a short URL by its code, checking cache first.
func (r *RedisCacheRepository) GetByCode(ctx context.Context, code shortener.Code) (*shortener.ShortURL, error) {
	// Check cache first
	if url, err := r.getFromCache(ctx, code); err == nil {
		return url, nil
	}

	// Cache miss - fetch from store
	url, err := r.store.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	// Populate cache
	r.cacheURL(ctx, url)

	return url, nil
}

// GetByHash retrieves a short URL by its hash, checking cache first.
func (r *RedisCacheRepository) GetByHash(ctx context.Context, hash shortener.URLHash) (*shortener.ShortURL, error) {
	// Check hash index cache first
	code, err := r.client.HGet(ctx, r.hashKey, string(hash)).Result()
	if err == nil {
		// Found code in hash index, try to get the full URL from cache
		if url, err := r.getFromCache(ctx, shortener.Code(code)); err == nil {
			return url, nil
		}
	}

	// Cache miss - fetch from store
	url, err := r.store.GetByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	// Populate cache
	r.cacheURL(ctx, url)

	return url, nil
}

func (r *RedisCacheRepository) getFromCache(ctx context.Context, code shortener.Code) (*shortener.ShortURL, error) {
	result, err := r.client.HGetAll(ctx, r.prefix+string(code)).Result()
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, shortener.ErrNotFound
	}

	var createdAt time.Time

	if ts, ok := result["created_at"]; ok {
		if nanos, err := strconv.ParseInt(ts, 10, 64); err == nil {
			createdAt = time.Unix(0, nanos)
		}
	}

	return &shortener.ShortURL{
		Code:        shortener.Code(result["code"]),
		OriginalURL: result["original_url"],
		URLHash:     shortener.URLHash(result["url_hash"]),
		CreatedAt:   createdAt,
	}, nil
}

func (r *RedisCacheRepository) cacheURL(ctx context.Context, url *shortener.ShortURL) {
	pipe := r.client.Pipeline()
	key := r.prefix + string(url.Code)

	pipe.HSet(ctx, key, map[string]interface{}{
		"code":         string(url.Code),
		"original_url": url.OriginalURL,
		"url_hash":     string(url.URLHash),
		"created_at":   url.CreatedAt.UnixNano(),
	})

	if r.ttl > 0 {
		pipe.Expire(ctx, key, r.ttl)
	}

	// Index by hash if present
	if url.URLHash != "" {
		pipe.HSet(ctx, r.hashKey, string(url.URLHash), string(url.Code))
	}

	_, _ = pipe.Exec(ctx)
}

// Shutdown is a no-op for RedisCacheRepository (client managed externally).
func (r *RedisCacheRepository) Shutdown() error {
	return nil
}

// Compile-time check.
var _ shortener.Repository = (*RedisCacheRepository)(nil)
