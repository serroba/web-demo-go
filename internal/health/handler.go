package health

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/redis/go-redis/v9"
)

// Checker defines the interface for checking service health.
type Checker interface {
	Ping(ctx context.Context) error
}

// RedisChecker adapts redis.Client to Checker interface.
type RedisChecker struct {
	client *redis.Client
}

// NewRedisChecker creates a new Redis health checker.
func NewRedisChecker(client *redis.Client) *RedisChecker {
	return &RedisChecker{client: client}
}

// Ping checks Redis connectivity.
func (r *RedisChecker) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Handler handles health check operations.
type Handler struct {
	redis Checker
}

// NewHandler creates a new health handler.
func NewHandler(redis Checker) *Handler {
	return &Handler{redis: redis}
}

// Response is the response for health check endpoint.
type Response struct {
	Body struct {
		Status string `json:"status"`
		Redis  string `json:"redis"`
	}
}

// Check performs a health check of the application and its dependencies.
func (h *Handler) Check(ctx context.Context, _ *struct{}) (*Response, error) {
	resp := &Response{}
	resp.Body.Status = "ok"

	if err := h.redis.Ping(ctx); err != nil {
		resp.Body.Redis = "unhealthy"
		resp.Body.Status = "degraded"
	} else {
		resp.Body.Redis = "healthy"
	}

	return resp, nil
}

// RegisterRoutes registers health check routes.
func RegisterRoutes(api huma.API, h *Handler) {
	huma.Get(api, "/health", h.Check)
}
