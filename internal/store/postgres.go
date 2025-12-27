package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/serroba/web-demo-go/internal/shortener"
)

// PostgresStore is a PostgreSQL implementation of shortener.Repository.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new PostgreSQL-backed URL store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (p *PostgresStore) Save(ctx context.Context, shortURL *shortener.ShortURL) error {
	query := `
		INSERT INTO short_urls (code, original_url, url_hash, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (code) DO NOTHING
	`

	_, err := p.pool.Exec(ctx, query,
		string(shortURL.Code),
		shortURL.OriginalURL,
		nullableString(shortURL.URLHash),
		shortURL.CreatedAt,
	)

	return err
}

func (p *PostgresStore) GetByCode(ctx context.Context, code shortener.Code) (*shortener.ShortURL, error) {
	query := `
		SELECT code, original_url, url_hash, created_at
		FROM short_urls
		WHERE code = $1
	`

	var url shortener.ShortURL

	var urlHash *string

	err := p.pool.QueryRow(ctx, query, string(code)).Scan(
		&url.Code,
		&url.OriginalURL,
		&urlHash,
		&url.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, shortener.ErrNotFound
		}

		return nil, err
	}

	if urlHash != nil {
		url.URLHash = shortener.URLHash(*urlHash)
	}

	return &url, nil
}

func (p *PostgresStore) GetByHash(ctx context.Context, hash shortener.URLHash) (*shortener.ShortURL, error) {
	query := `
		SELECT code, original_url, url_hash, created_at
		FROM short_urls
		WHERE url_hash = $1
	`

	var url shortener.ShortURL

	var urlHash *string

	err := p.pool.QueryRow(ctx, query, string(hash)).Scan(
		&url.Code,
		&url.OriginalURL,
		&urlHash,
		&url.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, shortener.ErrNotFound
		}

		return nil, err
	}

	if urlHash != nil {
		url.URLHash = shortener.URLHash(*urlHash)
	}

	return &url, nil
}

func nullableString(s shortener.URLHash) *string {
	if s == "" {
		return nil
	}

	str := string(s)

	return &str
}
