package store

import (
	"context"
	"net"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/serroba/web-demo-go/internal/analytics"
)

// Postgres persists analytics events to TimescaleDB hypertables.
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres creates a new PostgreSQL analytics store.
func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func (p *Postgres) SaveURLCreated(ctx context.Context, event *analytics.URLCreatedEvent) error {
	query := `
		INSERT INTO url_created_events (code, original_url, url_hash, strategy, created_at, client_ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := p.pool.Exec(ctx, query,
		event.Code,
		event.OriginalURL,
		nullableString(event.URLHash),
		event.Strategy,
		event.CreatedAt,
		parseIP(event.ClientIP),
		nullableString(event.UserAgent),
	)

	return err
}

func (p *Postgres) SaveURLAccessed(ctx context.Context, event *analytics.URLAccessedEvent) error {
	query := `
		INSERT INTO url_accessed_events (code, accessed_at, client_ip, user_agent, referrer)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := p.pool.Exec(ctx, query,
		event.Code,
		event.AccessedAt,
		parseIP(event.ClientIP),
		nullableString(event.UserAgent),
		nullableString(event.Referrer),
	)

	return err
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

func parseIP(s string) net.IP {
	if s == "" {
		return nil
	}

	return net.ParseIP(s)
}

// Compile-time check.
var _ analytics.Store = (*Postgres)(nil)
