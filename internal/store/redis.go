package store

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/serroba/web-demo-go/internal/shortener"
)

// RedisStore is a Redis implementation of shortener.Repository.
type RedisStore struct {
	client  *redis.Client
	prefix  string // "url:" prefix for code->entity (stored as Redis hash)
	hashKey string // "url_hashes" for urlHash->code lookup
}

// NewRedisStore creates a new Redis-backed URL store.
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{
		client:  client,
		prefix:  "url:",
		hashKey: "url_hashes",
	}
}

func (r *RedisStore) Save(ctx context.Context, shortURL *shortener.ShortURL) error {
	pipe := r.client.Pipeline()

	// Store entity as Redis hash
	pipe.HSet(ctx, r.prefix+string(shortURL.Code), map[string]interface{}{
		"code":         string(shortURL.Code),
		"original_url": shortURL.OriginalURL,
		"url_hash":     string(shortURL.URLHash),
		"created_at":   shortURL.CreatedAt.UnixNano(),
	})

	// Index by hash if present (for hash strategy)
	if shortURL.URLHash != "" {
		pipe.HSet(ctx, r.hashKey, string(shortURL.URLHash), string(shortURL.Code))
	}

	_, err := pipe.Exec(ctx)

	return err
}

func (r *RedisStore) GetByCode(ctx context.Context, code shortener.Code) (*shortener.ShortURL, error) {
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

func (r *RedisStore) GetByHash(ctx context.Context, hash shortener.URLHash) (*shortener.ShortURL, error) {
	code, err := r.client.HGet(ctx, r.hashKey, string(hash)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, shortener.ErrNotFound
		}

		return nil, err
	}

	return r.GetByCode(ctx, shortener.Code(code))
}
