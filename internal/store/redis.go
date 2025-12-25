package store

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
	"github.com/serroba/web-demo-go/internal/handlers"
)

// RedisStore is a Redis implementation of URLRepository.
type RedisStore struct {
	client  *redis.Client
	prefix  string // "url:" for code->url (string keys)
	hashKey string // "url_hashes" for urlHash->code (hash map)
}

// NewRedisStore creates a new Redis-backed URL store.
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{
		client:  client,
		prefix:  "url:",
		hashKey: "url_hashes",
	}
}

func (r *RedisStore) Save(ctx context.Context, code, url string) error {
	return r.client.Set(ctx, r.prefix+code, url, 0).Err()
}

func (r *RedisStore) Get(ctx context.Context, code string) (string, error) {
	url, err := r.client.Get(ctx, r.prefix+code).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", handlers.ErrNotFound
		}

		return "", err
	}

	return url, nil
}

func (r *RedisStore) SaveWithHash(ctx context.Context, code, url, urlHash string) error {
	// Use a pipeline for atomic multi-key write
	pipe := r.client.Pipeline()
	pipe.Set(ctx, r.prefix+code, url, 0)
	pipe.HSet(ctx, r.hashKey, urlHash, code)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *RedisStore) GetCodeByHash(ctx context.Context, urlHash string) (string, error) {
	code, err := r.client.HGet(ctx, r.hashKey, urlHash).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", handlers.ErrNotFound
		}

		return "", err
	}

	return code, nil
}
