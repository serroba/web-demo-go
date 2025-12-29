package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/serroba/web-demo-go/internal/ratelimit"
	"go.uber.org/zap"
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

// PolicyRateLimiter returns a Huma middleware that applies policy-based rate limiting.
// It uses a ScopeResolver to determine which scopes apply to each request,
// then checks all applicable limits from the policy.
//
// Per-endpoint configuration can be provided via operation metadata using
// ratelimit.MetadataKey. This allows endpoints to:
//   - Disable rate limiting entirely (Disabled: true)
//   - Override the scope detection (Scope: ratelimit.ScopeRead)
//   - Define custom limits (Limits: []ratelimit.LimitConfig{...})
func PolicyRateLimiter(
	api huma.API,
	limiter *ratelimit.PolicyLimiter,
	resolver ratelimit.ScopeResolver,
	logger *zap.Logger,
) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		path := getOperationPath(ctx)

		// Check for per-endpoint configuration
		if cfg := ratelimit.GetEndpointConfig(ctx); cfg != nil {
			if handleEndpointConfig(api, ctx, limiter, cfg, path, logger, next) {
				return
			}
		}

		// Default behavior: use policy-based rate limiting
		key := clientKey(ctx)
		scopes := resolver.Resolve(ctx)

		allowed, exceeded, err := limiter.Allow(ctx.Context(), key, scopes)
		if err != nil {
			logger.Error("rate limit check failed", zap.String("path", path), zap.Error(err))
			_ = huma.WriteErr(api, ctx, http.StatusInternalServerError, "internal server error", err)

			return
		}

		if !allowed {
			handleRateLimitExceeded(api, ctx, exceeded, path, logger)

			return
		}

		next(ctx)
	}
}

// getOperationPath extracts the path from the operation, if available.
func getOperationPath(ctx huma.Context) string {
	if op := ctx.Operation(); op != nil {
		return op.Path
	}

	return ""
}

// handleEndpointConfig processes per-endpoint rate limit configuration.
// Returns true if the request was handled (should return early), false to continue.
func handleEndpointConfig(
	api huma.API,
	ctx huma.Context,
	limiter *ratelimit.PolicyLimiter,
	cfg *ratelimit.EndpointConfig,
	path string,
	logger *zap.Logger,
	next func(huma.Context),
) bool {
	if cfg.Disabled {
		logger.Debug("rate limiting disabled for endpoint",
			zap.String("path", path), zap.String("method", ctx.Method()))
		next(ctx)

		return true
	}

	if len(cfg.Limits) > 0 {
		if !checkCustomLimits(api, ctx, limiter.Store(), cfg.Limits, logger) {
			return true
		}

		next(ctx)

		return true
	}

	return false
}

// handleRateLimitExceeded logs and responds to a rate limit exceeded condition.
func handleRateLimitExceeded(
	api huma.API,
	ctx huma.Context,
	exceeded *ratelimit.LimitExceeded,
	path string,
	logger *zap.Logger,
) {
	msg := "rate limit exceeded"
	if exceeded != nil {
		msg = fmt.Sprintf("rate limit exceeded: %s scope, %d/%d requests in %s",
			exceeded.Scope, exceeded.Count, exceeded.Config.Max, exceeded.Config.Window)
		logger.Warn("rate limit exceeded",
			zap.String("path", path),
			zap.String("method", ctx.Method()),
			zap.String("scope", string(exceeded.Scope)),
			zap.Int64("count", exceeded.Count),
			zap.Int64("max", exceeded.Config.Max),
			zap.Duration("window", exceeded.Config.Window),
			zap.String("client_ip", clientIP(ctx)),
		)
	}

	_ = huma.WriteErr(api, ctx, http.StatusTooManyRequests, msg)
}

// checkCustomLimits applies custom rate limits defined in endpoint config.
// Returns true if request is allowed, false if rate limited.
//
// Note: The rate limit key uses the operation's route template (e.g., "/{code}"),
// not the actual request path. This means all requests matching the same route
// pattern share rate limit counters per client, regardless of specific path values.
func checkCustomLimits(
	api huma.API,
	ctx huma.Context,
	store ratelimit.Store,
	limits []ratelimit.LimitConfig,
	logger *zap.Logger,
) bool {
	clientK := clientKey(ctx)

	op := ctx.Operation()
	if op == nil {
		logger.Error("missing operation in context for rate limiting")

		_ = huma.WriteErr(api, ctx, http.StatusInternalServerError, "internal server error",
			errors.New("missing operation in context"))

		return false
	}

	path := op.Path

	for _, limit := range limits {
		// Build key combining client, route template, and window for unique tracking
		key := fmt.Sprintf("%s:custom:%s:%d", clientK, path, limit.Window.Milliseconds())

		count, err := store.Record(ctx.Context(), key, limit.Window)
		if err != nil {
			logger.Error("custom rate limit check failed",
				zap.String("path", path),
				zap.Error(err),
			)
			_ = huma.WriteErr(api, ctx, http.StatusInternalServerError, "internal server error", err)

			return false
		}

		if count > limit.Max {
			logger.Warn("custom rate limit exceeded",
				zap.String("path", path),
				zap.String("method", ctx.Method()),
				zap.Int64("count", count),
				zap.Int64("max", limit.Max),
				zap.Duration("window", limit.Window),
				zap.String("client_ip", clientIP(ctx)),
			)
			msg := fmt.Sprintf("rate limit exceeded: %d/%d requests in %s",
				count, limit.Max, limit.Window)
			_ = huma.WriteErr(api, ctx, http.StatusTooManyRequests, msg)

			return false
		}
	}

	return true
}
