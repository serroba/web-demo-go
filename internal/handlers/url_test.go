package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jaevor/go-nanoid"
	"github.com/serroba/web-demo-go/internal/analytics"
	"github.com/serroba/web-demo-go/internal/handlers"
	"github.com/serroba/web-demo-go/internal/messaging"
	"github.com/serroba/web-demo-go/internal/shortener"
	"github.com/serroba/web-demo-go/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// noopPublish returns a publish function that always succeeds.
func noopPublish[T any]() messaging.Publish[T] {
	return func(_ *T) error { return nil }
}

// errorPublish returns a publish function that always fails.
func errorPublish[T any](err error) messaging.Publish[T] {
	return func(_ *T) error { return err }
}

func newTestHandler(s shortener.Repository) *handlers.URLHandler {
	gen, _ := nanoid.Standard(8)

	strategies := map[handlers.Strategy]shortener.Strategy{
		handlers.StrategyToken: shortener.NewTokenStrategy(s, gen),
		handlers.StrategyHash:  shortener.NewHashStrategy(s, gen),
	}

	return handlers.NewURLHandler(
		s,
		"http://localhost:8888",
		strategies,
		noopPublish[analytics.URLCreatedEvent](),
		noopPublish[analytics.URLAccessedEvent](),
		zap.NewNop(),
	)
}

func newTestHandlerWithPublishError(s shortener.Repository) *handlers.URLHandler {
	gen, _ := nanoid.Standard(8)

	strategies := map[handlers.Strategy]shortener.Strategy{
		handlers.StrategyToken: shortener.NewTokenStrategy(s, gen),
		handlers.StrategyHash:  shortener.NewHashStrategy(s, gen),
	}

	return handlers.NewURLHandler(
		s,
		"http://localhost:8888",
		strategies,
		errorPublish[analytics.URLCreatedEvent](errors.New("publish error")),
		errorPublish[analytics.URLAccessedEvent](errors.New("publish error")),
		zap.NewNop(),
	)
}

func TestCreateShortURL(t *testing.T) {
	t.Run("creates short url successfully", func(t *testing.T) {
		memStore := store.NewMemoryStore()
		handler := newTestHandler(memStore)

		req := &handlers.CreateShortURLRequest{}
		req.Body.URL = "https://example.com/very/long/path"

		resp, err := handler.CreateShortURL(context.Background(), req)

		require.NoError(t, err)
		assert.NotEmpty(t, resp.Body.Code)
		assert.Equal(t, "https://example.com/very/long/path", resp.Body.OriginalURL)
		assert.Contains(t, resp.Body.ShortURL, resp.Body.Code)
		assert.Equal(t, resp.Body.ShortURL, resp.Headers.Location)
	})

	t.Run("returns error for invalid strategy", func(t *testing.T) {
		memStore := store.NewMemoryStore()
		handler := newTestHandler(memStore)

		req := &handlers.CreateShortURLRequest{}
		req.Body.URL = testURL
		req.Body.Strategy = "invalid"

		resp, err := handler.CreateShortURL(context.Background(), req)

		assert.Nil(t, resp)
		assert.Error(t, err)
	})

	t.Run("token strategy creates new code for same URL", func(t *testing.T) {
		memStore := store.NewMemoryStore()
		handler := newTestHandler(memStore)

		req := &handlers.CreateShortURLRequest{}
		req.Body.URL = testURL
		req.Body.Strategy = handlers.StrategyToken

		resp1, err1 := handler.CreateShortURL(context.Background(), req)
		resp2, err2 := handler.CreateShortURL(context.Background(), req)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, resp1.Body.Code, resp2.Body.Code)
	})

	t.Run("hash strategy returns same code for same URL", func(t *testing.T) {
		memStore := store.NewMemoryStore()
		handler := newTestHandler(memStore)

		req := &handlers.CreateShortURLRequest{}
		req.Body.URL = testURL
		req.Body.Strategy = handlers.StrategyHash

		resp1, err1 := handler.CreateShortURL(context.Background(), req)
		resp2, err2 := handler.CreateShortURL(context.Background(), req)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, resp1.Body.Code, resp2.Body.Code)
	})

	t.Run("hash strategy returns same code for equivalent URLs", func(t *testing.T) {
		memStore := store.NewMemoryStore()
		handler := newTestHandler(memStore)

		req1 := &handlers.CreateShortURLRequest{}
		req1.Body.URL = "https://example.com/path"
		req1.Body.Strategy = handlers.StrategyHash

		req2 := &handlers.CreateShortURLRequest{}
		req2.Body.URL = "https://example.com/path/"
		req2.Body.Strategy = handlers.StrategyHash

		resp1, err1 := handler.CreateShortURL(context.Background(), req1)
		resp2, err2 := handler.CreateShortURL(context.Background(), req2)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, resp1.Body.Code, resp2.Body.Code)
	})

	t.Run("hash strategy returns different codes for different URLs", func(t *testing.T) {
		memStore := store.NewMemoryStore()
		handler := newTestHandler(memStore)

		req1 := &handlers.CreateShortURLRequest{}
		req1.Body.URL = "https://example.com/path1"
		req1.Body.Strategy = handlers.StrategyHash

		req2 := &handlers.CreateShortURLRequest{}
		req2.Body.URL = "https://example.com/path2"
		req2.Body.Strategy = handlers.StrategyHash

		resp1, err1 := handler.CreateShortURL(context.Background(), req1)
		resp2, err2 := handler.CreateShortURL(context.Background(), req2)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, resp1.Body.Code, resp2.Body.Code)
	})

	t.Run("defaults to token strategy when not specified", func(t *testing.T) {
		memStore := store.NewMemoryStore()
		handler := newTestHandler(memStore)

		req := &handlers.CreateShortURLRequest{}
		req.Body.URL = testURL
		// Strategy not set - should default to token

		resp1, err1 := handler.CreateShortURL(context.Background(), req)
		resp2, err2 := handler.CreateShortURL(context.Background(), req)

		require.NoError(t, err1)
		require.NoError(t, err2)
		// Token strategy: different codes for same URL
		assert.NotEqual(t, resp1.Body.Code, resp2.Body.Code)
	})
}

func TestRedirectToURL(t *testing.T) {
	t.Run("redirects to original url", func(t *testing.T) {
		memStore := store.NewMemoryStore()
		_ = memStore.Save(context.Background(), &shortener.ShortURL{
			Code:        "abc123",
			OriginalURL: testURL,
		})
		handler := newTestHandler(memStore)

		req := &handlers.RedirectRequest{Code: "abc123"}

		resp, err := handler.RedirectToURL(context.Background(), req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusMovedPermanently, resp.Status)
		assert.Equal(t, testURL, resp.Headers.Location)
	})

	t.Run("returns 404 when code not found", func(t *testing.T) {
		memStore := store.NewMemoryStore()
		handler := newTestHandler(memStore)

		req := &handlers.RedirectRequest{Code: "notfound"}

		resp, err := handler.RedirectToURL(context.Background(), req)

		assert.Nil(t, resp)
		assert.Error(t, err)
	})

	t.Run("returns 500 on store error", func(t *testing.T) {
		mockStore := &mockStore{getByCodeErr: errMock}
		handler := newTestHandler(mockStore)

		req := &handlers.RedirectRequest{Code: "abc123"}

		resp, err := handler.RedirectToURL(context.Background(), req)

		assert.Nil(t, resp)
		assert.Error(t, err)
	})
}

func TestCreateShortURL_ErrorPaths(t *testing.T) {
	t.Run("token strategy returns error when save fails", func(t *testing.T) {
		mockStore := &mockStore{
			saveErr:      errMock,
			getByHashErr: shortener.ErrNotFound,
		}
		handler := newTestHandler(mockStore)

		req := &handlers.CreateShortURLRequest{}
		req.Body.URL = testURL
		req.Body.Strategy = handlers.StrategyToken

		resp, err := handler.CreateShortURL(context.Background(), req)

		assert.Nil(t, resp)
		assert.Error(t, err)
	})

	t.Run("hash strategy returns error on unexpected GetByHash error", func(t *testing.T) {
		mockStore := &mockStore{getByHashErr: errMock}
		handler := newTestHandler(mockStore)

		req := &handlers.CreateShortURLRequest{}
		req.Body.URL = testURL
		req.Body.Strategy = handlers.StrategyHash

		resp, err := handler.CreateShortURL(context.Background(), req)

		assert.Nil(t, resp)
		assert.Error(t, err)
	})

	t.Run("hash strategy returns error when Save fails", func(t *testing.T) {
		mockStore := &mockStore{
			getByHashErr: shortener.ErrNotFound,
			saveErr:      errMock,
		}
		handler := newTestHandler(mockStore)

		req := &handlers.CreateShortURLRequest{}
		req.Body.URL = testURL
		req.Body.Strategy = handlers.StrategyHash

		resp, err := handler.CreateShortURL(context.Background(), req)

		assert.Nil(t, resp)
		assert.Error(t, err)
	})
}

func TestContextWithRequestMeta(t *testing.T) {
	t.Run("adds and retrieves request metadata from context", func(t *testing.T) {
		meta := handlers.RequestMeta{
			ClientIP:  "192.168.1.1",
			UserAgent: "TestAgent/1.0",
			Referrer:  "https://referrer.com",
		}
		ctx := handlers.ContextWithRequestMeta(context.Background(), meta)

		retrieved := handlers.RequestMetaFromContext(ctx)
		assert.Equal(t, meta, retrieved)
	})
}

func TestCreateShortURL_WithRequestMeta(t *testing.T) {
	t.Run("uses request metadata from context", func(t *testing.T) {
		memStore := store.NewMemoryStore()
		handler := newTestHandler(memStore)

		meta := handlers.RequestMeta{
			ClientIP:  "192.168.1.1",
			UserAgent: "TestAgent/1.0",
			Referrer:  "https://referrer.com",
		}
		ctx := handlers.ContextWithRequestMeta(context.Background(), meta)

		req := &handlers.CreateShortURLRequest{}
		req.Body.URL = "https://example.com"

		resp, err := handler.CreateShortURL(ctx, req)

		require.NoError(t, err)
		assert.NotEmpty(t, resp.Body.Code)
	})
}

func TestCreateShortURL_PublishError(t *testing.T) {
	t.Run("succeeds even when publish fails", func(t *testing.T) {
		memStore := store.NewMemoryStore()
		handler := newTestHandlerWithPublishError(memStore)

		req := &handlers.CreateShortURLRequest{}
		req.Body.URL = "https://example.com"

		resp, err := handler.CreateShortURL(context.Background(), req)

		// Should succeed - publish errors are logged, not returned
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Body.Code)
	})
}

func TestRedirectToURL_WithRequestMeta(t *testing.T) {
	t.Run("uses request metadata from context", func(t *testing.T) {
		memStore := store.NewMemoryStore()
		_ = memStore.Save(context.Background(), &shortener.ShortURL{
			Code:        "abc123",
			OriginalURL: testURL,
		})
		handler := newTestHandler(memStore)

		meta := handlers.RequestMeta{
			ClientIP:  "192.168.1.1",
			UserAgent: "TestAgent/1.0",
			Referrer:  "https://referrer.com",
		}
		ctx := handlers.ContextWithRequestMeta(context.Background(), meta)

		req := &handlers.RedirectRequest{Code: "abc123"}

		resp, err := handler.RedirectToURL(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusMovedPermanently, resp.Status)
	})
}

func TestRedirectToURL_PublishError(t *testing.T) {
	t.Run("succeeds even when publish fails", func(t *testing.T) {
		memStore := store.NewMemoryStore()
		_ = memStore.Save(context.Background(), &shortener.ShortURL{
			Code:        "abc123",
			OriginalURL: testURL,
		})
		handler := newTestHandlerWithPublishError(memStore)

		req := &handlers.RedirectRequest{Code: "abc123"}

		resp, err := handler.RedirectToURL(context.Background(), req)

		// Should succeed - publish errors are logged, not returned
		require.NoError(t, err)
		assert.Equal(t, http.StatusMovedPermanently, resp.Status)
	})
}
