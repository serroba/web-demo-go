package middleware

import (
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/serroba/web-demo-go/internal/handlers"
)

// RequestMeta is a middleware that adds client IP, user-agent, and referrer to the request context.
func RequestMeta(_ huma.API) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		meta := handlers.RequestMeta{
			ClientIP:  extractClientIP(ctx),
			UserAgent: ctx.Header("User-Agent"),
			Referrer:  ctx.Header("Referer"),
		}

		newCtx := handlers.ContextWithRequestMeta(ctx.Context(), meta)
		ctx = huma.WithContext(ctx, newCtx)

		next(ctx)
	}
}

func extractClientIP(ctx huma.Context) string {
	// Check X-Forwarded-For first (may contain multiple IPs)
	if xff := ctx.Header("X-Forwarded-For"); xff != "" {
		// Take the first IP (original client)
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}

		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP
	if xri := ctx.Header("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to remote addr
	host := ctx.Host()
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		return host[:idx]
	}

	return host
}
