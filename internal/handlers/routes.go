package handlers

import (
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/serroba/web-demo-go/internal/ratelimit"
)

// RegisterRoutes registers all URL shortener routes with per-endpoint rate limit configuration.
func RegisterRoutes(api huma.API, urlHandler *URLHandler) {
	// POST /shorten - Create short URL
	// Uses stricter rate limits for write operations
	huma.Register(api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/shorten",
		Summary:     "Create short URL",
		Description: "Creates a shortened URL using the specified strategy (token or hash).",
		Tags:        []string{"URLs"},
		Metadata: map[string]any{
			ratelimit.MetadataKey: ratelimit.EndpointConfig{
				Limits: []ratelimit.LimitConfig{
					{Window: time.Minute, Max: 10},     // 10 per minute
					{Window: time.Hour, Max: 100},      // 100 per hour
					{Window: 24 * time.Hour, Max: 500}, // 500 per day
				},
			},
		},
	}, urlHandler.CreateShortURL)

	// GET /{code} - Redirect to original URL
	// Uses relaxed rate limits for high-traffic read operations
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/{code}",
		Summary:     "Redirect to original URL",
		Description: "Redirects to the original URL associated with the short code.",
		Tags:        []string{"URLs"},
		Metadata: map[string]any{
			ratelimit.MetadataKey: ratelimit.EndpointConfig{
				Limits: []ratelimit.LimitConfig{
					{Window: time.Minute, Max: 1000}, // 1000 per minute
				},
			},
		},
	}, urlHandler.RedirectToURL)
}
