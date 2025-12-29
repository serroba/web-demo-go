# URL Shortener - High-Performance Go Microservice

[![CI/CD Pipeline](https://github.com/serroba/web-demo-go/actions/workflows/ci.yml/badge.svg)](https://github.com/serroba/web-demo-go/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/serroba/web-demo-go/branch/main/graph/badge.svg)](https://codecov.io/gh/serroba/web-demo-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/serroba/web-demo-go)](https://goreportcard.com/report/github.com/serroba/web-demo-go)
[![Go Reference](https://pkg.go.dev/badge/github.com/serroba/web-demo-go.svg)](https://pkg.go.dev/github.com/serroba/web-demo-go)

A production-ready URL shortening microservice built with Go, featuring distributed rate limiting, event-driven analytics, and time-series data storage. Designed for high throughput and horizontal scalability.

## Features

- **Multiple Shortening Strategies** - Token-based (unique per request) or hash-based (URL deduplication)
- **Policy-Based Rate Limiting** - Configurable limits per scope (read/write) with sliding window algorithm
- **Dual Storage Backend** - In-memory for development, Redis for distributed deployments
- **Event-Driven Architecture** - Async analytics via Redis Streams with Watermill
- **Time-Series Analytics** - URL creation and access events stored in TimescaleDB
- **Multi-Layer Caching** - LRU in-memory cache with Redis cache-aside pattern
- **OpenAPI Documentation** - Auto-generated API docs with Huma framework
- **Health Checks** - Kubernetes-ready liveness and readiness probes

## Tech Stack

| Component      | Technology                                                |
|----------------|-----------------------------------------------------------|
| Language       | Go 1.25+                                                  |
| HTTP Framework | [Huma](https://huma.rocks/) with Chi router               |
| Database       | PostgreSQL with [TimescaleDB](https://www.timescale.com/) |
| Cache          | Redis 7                                                   |
| Messaging      | Redis Streams via [Watermill](https://watermill.io/)      |
| Migrations     | [Atlas](https://atlasgo.io/)                              |
| DI Container   | [samber/do](https://github.com/samber/do)                 |

## Quick Start

### Prerequisites

- Go 1.25+
- Docker and Docker Compose

### Run Locally

```bash
# Start dependencies (Redis, TimescaleDB) and run migrations
docker-compose up -d

# Start the server
go run ./cmd/server --database-url="postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable"
```

The API will be available at `http://localhost:8888`.

## API Reference

### Create Short URL

```http
POST /shorten
Content-Type: application/json

{
  "url": "https://example.com/very/long/path",
  "strategy": "token"
}
```

**Strategies:**
| Strategy | Description |
|----------|-------------|
| `token` | Generates a unique short code for every request (default) |
| `hash` | Returns the same short code for identical URLs (deduplication) |

**Response:**
```json
{
  "code": "abc123",
  "shortUrl": "http://localhost:8888/abc123",
  "originalUrl": "https://example.com/very/long/path"
}
```

### Redirect

```http
GET /{code}
```

Returns a `301 Moved Permanently` redirect to the original URL.

### Health Check

```http
GET /health
```

Returns service health status including Redis connectivity.

## Configuration

All settings can be configured via environment variables or command-line flags:

| Environment Variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `DATABASE_URL` | `--database-url` | - | PostgreSQL connection string (required) |
| `RATE_LIMIT_STORE` | `--rate-limit-store` | `memory` | Rate limit backend (`memory` or `redis`) |
| `RATE_LIMIT_GLOBAL_DAY` | `--rate-limit-global-per-day` | `5000` | Max requests per day (global) |
| `RATE_LIMIT_READ_MINUTE` | `--rate-limit-read-per-minute` | `100` | Max read requests per minute |
| `RATE_LIMIT_WRITE_MINUTE` | `--rate-limit-write-per-minute` | `10` | Max write requests per minute |
| `CACHE_SIZE` | `--cache-size` | `1000` | LRU cache size (0 to disable) |
| `CACHE_TTL` | `--cache-ttl` | `1h` | Redis cache TTL |

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Client    │────▶│  Rate Limit │────▶│   Handler   │
└─────────────┘     │  Middleware │     └──────┬──────┘
                    └─────────────┘            │
                           │                   ▼
                    ┌──────┴──────┐     ┌─────────────┐
                    │    Redis    │     │  Repository │
                    │   (Store)   │     └──────┬──────┘
                    └─────────────┘            │
                                         ┌────┴────┐
                                         ▼         ▼
                                   ┌─────────┐ ┌─────────┐
                                   │  Redis  │ │ Postgres│
                                   │ (Cache) │ │  (DB)   │
                                   └─────────┘ └─────────┘

┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Handler   │────▶│   Redis     │────▶│  Consumer   │────▶ TimescaleDB
│  (Events)   │     │  Streams    │     │  (Analytics)│
└─────────────┘     └─────────────┘     └─────────────┘
```

## Development

```bash
# Run tests
go test ./...

# Run tests with coverage
go test ./... -coverprofile=coverage.out

# Run linter
golangci-lint run

# Generate mocks
go generate ./...
```

## License

MIT License - see [LICENSE](LICENSE) for details.
