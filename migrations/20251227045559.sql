-- URL creation events for strategy analysis
CREATE TABLE url_created_events (
    code         VARCHAR(16) NOT NULL,
    original_url TEXT NOT NULL,
    url_hash     VARCHAR(64),
    strategy     VARCHAR(16) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL,
    client_ip    INET,
    user_agent   TEXT
);

SELECT create_hypertable('url_created_events', by_range('created_at', INTERVAL '7 days'));

CREATE INDEX idx_url_created_code ON url_created_events (code, created_at DESC);
CREATE INDEX idx_url_created_strategy ON url_created_events (strategy, created_at DESC);

-- URL access events for usage tracking
CREATE TABLE url_accessed_events (
    code        VARCHAR(16) NOT NULL,
    accessed_at TIMESTAMPTZ NOT NULL,
    client_ip   INET,
    user_agent  TEXT,
    referrer    TEXT
);

SELECT create_hypertable('url_accessed_events', by_range('accessed_at', INTERVAL '7 days'));

CREATE INDEX idx_url_accessed_code ON url_accessed_events (code, accessed_at DESC);
