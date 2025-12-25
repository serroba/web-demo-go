package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/serroba/web-demo-go/internal/ratelimit"
)

// RateLimiter returns a Huma middleware that limits requests based on client IP and User-Agent.
func RateLimiter(api huma.API, limiter ratelimit.Limiter) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		key := clientKey(ctx)

		allowed, err := limiter.Allow(ctx.Context(), key)
		if err != nil {
			_ = huma.WriteErr(api, ctx, http.StatusInternalServerError, "internal server error", err)

			return
		}

		if !allowed {
			_ = huma.WriteErr(api, ctx, http.StatusTooManyRequests, "rate limit exceeded")

			return
		}

		next(ctx)
	}
}

// clientKey generates a unique key for rate limiting based on IP and User-Agent.
func clientKey(ctx huma.Context) string {
	ip := clientIP(ctx)
	ua := ctx.Header("User-Agent")

	hash := sha256.Sum256([]byte(ip + "|" + ua))

	return hex.EncodeToString(hash[:])
}

// clientIP extracts the client IP from the request, considering proxies.
func clientIP(ctx huma.Context) string {
	// Check X-Forwarded-For header (may contain multiple IPs)
	if xff := ctx.Header("X-Forwarded-For"); xff != "" {
		// Take the first IP (original client)
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}

		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := ctx.Header("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to Host (which contains remote addr in Huma context)
	host := ctx.Host()

	ip, _, err := net.SplitHostPort(host)
	if err != nil {
		return host
	}

	return ip
}
