# URL Shortener

A URL shortening service with two strategies for generating short codes.

## API

### Create Short URL

```
POST /shorten
```

**Request:**
```json
{
  "url": "https://example.com/very/long/path",
  "strategy": "token"
}
```

**Strategies:**
- `token` (default): Generates a unique short code for every request
- `hash`: Returns the same short code for identical URLs (deduplication)

**Response:**
```json
{
  "code": "abc123",
  "shortUrl": "http://localhost:8888/abc123",
  "originalUrl": "https://example.com/very/long/path"
}
```

### Redirect

```
GET /{code}
```

Redirects (301) to the original URL.
