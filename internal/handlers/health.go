package handlers

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// HealthChecker defines the interface for checking service health.
type HealthChecker interface {
	Ping(ctx context.Context) error
}

// RedisHealthChecker adapts redis.Client to HealthChecker interface.
type RedisHealthChecker struct {
	client *redis.Client
}

// NewRedisHealthChecker creates a new Redis health checker.
func NewRedisHealthChecker(client *redis.Client) *RedisHealthChecker {
	return &RedisHealthChecker{client: client}
}

// Ping checks Redis connectivity.
func (r *RedisHealthChecker) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// HealthHandler handles health check operations.
type HealthHandler struct {
	redis HealthChecker
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(redis HealthChecker) *HealthHandler {
	return &HealthHandler{redis: redis}
}

// HealthResponse is the response for health check endpoint.
type HealthResponse struct {
	Body struct {
		Status string `json:"status"`
		Redis  string `json:"redis"`
	}
}

// Check performs a health check of the application and its dependencies.
func (h *HealthHandler) Check(ctx context.Context, _ *struct{}) (*HealthResponse, error) {
	resp := &HealthResponse{}
	resp.Body.Status = "ok"

	if err := h.redis.Ping(ctx); err != nil {
		resp.Body.Redis = "unhealthy"
		resp.Body.Status = "degraded"
	} else {
		resp.Body.Redis = "healthy"
	}

	return resp, nil
}
