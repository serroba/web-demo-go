package middleware_test

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"mime/multipart"
	"net/url"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/serroba/web-demo-go/internal/middleware"
	"github.com/stretchr/testify/assert"
)

const (
	testHostAddr  = "192.168.1.1:12345"
	testUserAgent = "TestAgent/1.0"
)

var errMultipartNotSupported = errors.New("multipart not supported in mock")

func newTestAPI() huma.API {
	return humachi.New(chi.NewMux(), huma.DefaultConfig("Test", "1.0.0"))
}

type mockLimiter struct {
	allowed bool
	err     error
}

func (m *mockLimiter) Allow(_ context.Context, _ string) (bool, error) {
	return m.allowed, m.err
}

// mockHumaContext implements huma.Context for testing.
type mockHumaContext struct {
	headers    map[string]string
	host       string
	remoteAddr string
	written    []byte
	statusCode int
}

func newMockHumaContext() *mockHumaContext {
	return &mockHumaContext{
		headers: make(map[string]string),
	}
}

func (m *mockHumaContext) Operation() *huma.Operation            { return nil }
func (m *mockHumaContext) Context() context.Context              { return context.Background() }
func (m *mockHumaContext) TLS() *tls.ConnectionState             { return nil }
func (m *mockHumaContext) Version() huma.ProtoVersion            { return huma.ProtoVersion{} }
func (m *mockHumaContext) Method() string                        { return "GET" }
func (m *mockHumaContext) Host() string                          { return m.host }
func (m *mockHumaContext) RemoteAddr() string                    { return m.remoteAddr }
func (m *mockHumaContext) URL() url.URL                          { return url.URL{} }
func (m *mockHumaContext) Param(_ string) string                 { return "" }
func (m *mockHumaContext) Query(_ string) string                 { return "" }
func (m *mockHumaContext) Header(name string) string             { return m.headers[name] }
func (m *mockHumaContext) EachHeader(_ func(name, value string)) {}
func (m *mockHumaContext) BodyReader() io.Reader                 { return nil }
func (m *mockHumaContext) GetMultipartForm() (*multipart.Form, error) {
	return nil, errMultipartNotSupported
}
func (m *mockHumaContext) SetReadDeadline(_ time.Time) error { return nil }
func (m *mockHumaContext) SetStatus(code int)                { m.statusCode = code }
func (m *mockHumaContext) Status() int                       { return m.statusCode }
func (m *mockHumaContext) AppendHeader(_, _ string)          {}
func (m *mockHumaContext) SetHeader(_, _ string)             {}
func (m *mockHumaContext) BodyWriter() io.Writer             { return &mockBodyWriter{ctx: m} }

type mockBodyWriter struct {
	ctx *mockHumaContext
}

func (w *mockBodyWriter) Write(p []byte) (n int, err error) {
	w.ctx.written = append(w.ctx.written, p...)

	return len(p), nil
}

func TestRateLimiter(t *testing.T) {
	t.Run("allows request when limiter allows", func(t *testing.T) {
		api := newTestAPI()
		limiter := &mockLimiter{allowed: true}
		mw := middleware.RateLimiter(api, limiter)

		ctx := newMockHumaContext()
		ctx.host = testHostAddr
		ctx.headers["User-Agent"] = testUserAgent

		nextCalled := false

		mw(ctx, func(_ huma.Context) {
			nextCalled = true
		})

		assert.True(t, nextCalled, "next should be called when allowed")
	})

	t.Run("returns 429 when rate limited", func(t *testing.T) {
		api := newTestAPI()
		limiter := &mockLimiter{allowed: false}
		mw := middleware.RateLimiter(api, limiter)

		ctx := newMockHumaContext()
		ctx.host = testHostAddr
		ctx.headers["User-Agent"] = testUserAgent

		nextCalled := false

		mw(ctx, func(_ huma.Context) {
			nextCalled = true
		})

		assert.False(t, nextCalled, "next should not be called when rate limited")
		assert.Equal(t, 429, ctx.statusCode)
		assert.Contains(t, string(ctx.written), "rate limit")
	})

	t.Run("uses IP and User-Agent for client key", func(t *testing.T) {
		api := newTestAPI()

		var capturedKey string

		limiter := &capturingLimiter{
			allowed:     true,
			capturedKey: &capturedKey,
		}
		mw := middleware.RateLimiter(api, limiter)

		ctx1 := newMockHumaContext()
		ctx1.host = testHostAddr
		ctx1.headers["User-Agent"] = testUserAgent

		mw(ctx1, func(_ huma.Context) {})

		key1 := capturedKey

		ctx2 := newMockHumaContext()
		ctx2.host = testHostAddr
		ctx2.headers["User-Agent"] = testUserAgent

		mw(ctx2, func(_ huma.Context) {})

		key2 := capturedKey

		assert.Equal(t, key1, key2, "same IP and User-Agent should produce same key")

		// Different User-Agent should produce different key
		ctx3 := newMockHumaContext()
		ctx3.host = testHostAddr
		ctx3.headers["User-Agent"] = "DifferentAgent/2.0"

		mw(ctx3, func(_ huma.Context) {})

		key3 := capturedKey

		assert.NotEqual(t, key1, key3, "different User-Agent should produce different key")
	})

	t.Run("extracts IP from X-Forwarded-For header", func(t *testing.T) {
		api := newTestAPI()

		var capturedKey string

		limiter := &capturingLimiter{
			allowed:     true,
			capturedKey: &capturedKey,
		}
		mw := middleware.RateLimiter(api, limiter)

		ctx := newMockHumaContext()
		ctx.host = "10.0.0.1:12345"
		ctx.headers["X-Forwarded-For"] = "203.0.113.195, 70.41.3.18, 150.172.238.178"
		ctx.headers["User-Agent"] = "TestAgent"

		mw(ctx, func(_ huma.Context) {})

		keyWithXFF := capturedKey

		// Request with same first XFF IP should have same key
		ctx2 := newMockHumaContext()
		ctx2.host = "10.0.0.2:54321"
		ctx2.headers["X-Forwarded-For"] = "203.0.113.195"
		ctx2.headers["User-Agent"] = "TestAgent"

		mw(ctx2, func(_ huma.Context) {})

		assert.Equal(t, keyWithXFF, capturedKey, "should use first IP from X-Forwarded-For")
	})
}

type capturingLimiter struct {
	allowed     bool
	capturedKey *string
}

func (c *capturingLimiter) Allow(_ context.Context, key string) (bool, error) {
	*c.capturedKey = key

	return c.allowed, nil
}
