package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/serroba/web-demo-go/internal/middleware"
	"github.com/stretchr/testify/assert"
)

type testOutput struct {
	Body string `json:"body"`
}

func setupTestAPI(t *testing.T) (*chi.Mux, huma.API) {
	t.Helper()

	router := chi.NewMux()
	api := humachi.New(router, huma.DefaultConfig("Test", "1.0.0"))
	api.UseMiddleware(middleware.RequestMeta(api))

	return router, api
}

func TestRequestMeta(t *testing.T) {
	t.Run("extracts user-agent and referrer", func(t *testing.T) {
		router, api := setupTestAPI(t)

		ctxChan := make(chan context.Context, 1)

		huma.Get(api, "/test", func(ctx context.Context, _ *struct{}) (*testOutput, error) {
			ctxChan <- ctx

			return &testOutput{Body: "ok"}, nil
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("User-Agent", "TestAgent/1.0")
		req.Header.Set("Referer", "https://example.com")

		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		capturedCtx := <-ctxChan
		assert.NotNil(t, capturedCtx)
	})

	t.Run("extracts IP from X-Forwarded-For with single IP", func(t *testing.T) {
		router, api := setupTestAPI(t)

		huma.Get(api, "/test", func(_ context.Context, _ *struct{}) (*testOutput, error) {
			return &testOutput{Body: "ok"}, nil
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.1")

		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("extracts first IP from X-Forwarded-For with multiple IPs", func(t *testing.T) {
		router, api := setupTestAPI(t)

		huma.Get(api, "/test", func(_ context.Context, _ *struct{}) (*testOutput, error) {
			return &testOutput{Body: "ok"}, nil
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1, 172.16.0.1")

		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("extracts IP from X-Real-IP when X-Forwarded-For is absent", func(t *testing.T) {
		router, api := setupTestAPI(t)

		huma.Get(api, "/test", func(_ context.Context, _ *struct{}) (*testOutput, error) {
			return &testOutput{Body: "ok"}, nil
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Real-IP", "10.0.0.1")

		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("falls back to host when no IP headers present", func(t *testing.T) {
		router, api := setupTestAPI(t)

		huma.Get(api, "/test", func(_ context.Context, _ *struct{}) (*testOutput, error) {
			return &testOutput{Body: "ok"}, nil
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)

		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}
