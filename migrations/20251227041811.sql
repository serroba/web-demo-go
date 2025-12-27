-- Short URLs table (source of truth for URL mappings)
CREATE TABLE short_urls (
    code         VARCHAR(16) PRIMARY KEY,
    original_url TEXT NOT NULL,
    url_hash     VARCHAR(64),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for hash-based lookups (deduplication)
CREATE INDEX idx_short_urls_url_hash ON short_urls (url_hash) WHERE url_hash IS NOT NULL;
