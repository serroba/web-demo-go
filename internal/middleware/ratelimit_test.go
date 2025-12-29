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
	"github.com/serroba/web-demo-go/internal/ratelimit"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

const (
	testHostAddr       = "192.168.1.1:12345"
	testUserAgent      = "TestAgent/1.0"
	testUserAgentShort = "TestAgent"
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
	method     string
	operation  *huma.Operation
}

func newMockHumaContext() *mockHumaContext {
	return &mockHumaContext{
		headers: make(map[string]string),
		method:  "GET",
	}
}

func (m *mockHumaContext) Operation() *huma.Operation {
	return m.operation
}
func (m *mockHumaContext) Context() context.Context              { return context.Background() }
func (m *mockHumaContext) TLS() *tls.ConnectionState             { return nil }
func (m *mockHumaContext) Version() huma.ProtoVersion            { return huma.ProtoVersion{} }
func (m *mockHumaContext) Method() string                        { return m.method }
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
		ctx.headers["User-Agent"] = testUserAgentShort

		mw(ctx, func(_ huma.Context) {})

		keyWithXFF := capturedKey

		// Request with same first XFF IP should have same key
		ctx2 := newMockHumaContext()
		ctx2.host = "10.0.0.2:54321"
		ctx2.headers["X-Forwarded-For"] = "203.0.113.195"
		ctx2.headers["User-Agent"] = testUserAgentShort

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

func TestRateLimiter_LimiterError(t *testing.T) {
	api := newTestAPI()
	limiter := &mockLimiter{allowed: false, err: errors.New("limiter error")}
	mw := middleware.RateLimiter(api, limiter)

	ctx := newMockHumaContext()
	ctx.host = testHostAddr
	ctx.headers["User-Agent"] = testUserAgent

	nextCalled := false

	mw(ctx, func(_ huma.Context) {
		nextCalled = true
	})

	assert.False(t, nextCalled, "next should not be called when limiter errors")
	assert.Equal(t, 500, ctx.statusCode)
}

func TestClientIP_XRealIP(t *testing.T) {
	api := newTestAPI()

	var capturedKey string

	limiter := &capturingLimiter{
		allowed:     true,
		capturedKey: &capturedKey,
	}
	mw := middleware.RateLimiter(api, limiter)

	ctx := newMockHumaContext()
	ctx.host = "10.0.0.1:12345"
	ctx.headers["X-Real-IP"] = "203.0.113.100"
	ctx.headers["User-Agent"] = testUserAgentShort

	mw(ctx, func(_ huma.Context) {})

	keyWithXRI := capturedKey

	// Request with same X-Real-IP should have same key
	ctx2 := newMockHumaContext()
	ctx2.host = "10.0.0.2:54321"
	ctx2.headers["X-Real-IP"] = "203.0.113.100"
	ctx2.headers["User-Agent"] = testUserAgentShort

	mw(ctx2, func(_ huma.Context) {})

	assert.Equal(t, keyWithXRI, capturedKey, "should use X-Real-IP when present")
}

func TestClientIP_HostWithoutPort(t *testing.T) {
	api := newTestAPI()

	var capturedKey string

	limiter := &capturingLimiter{
		allowed:     true,
		capturedKey: &capturedKey,
	}
	mw := middleware.RateLimiter(api, limiter)

	// Host without port (SplitHostPort will fail)
	ctx := newMockHumaContext()
	ctx.host = "192.168.1.1"
	ctx.headers["User-Agent"] = testUserAgentShort

	mw(ctx, func(_ huma.Context) {})

	key1 := capturedKey

	// Same host should produce same key
	ctx2 := newMockHumaContext()
	ctx2.host = "192.168.1.1"
	ctx2.headers["User-Agent"] = testUserAgentShort

	mw(ctx2, func(_ huma.Context) {})

	assert.Equal(t, key1, capturedKey, "should use host as-is when SplitHostPort fails")
}

// mockPolicyStore is a mock store for testing PolicyRateLimiter.
type mockPolicyStore struct {
	counts map[string]int64
	err    error
}

func newMockPolicyStore() *mockPolicyStore {
	return &mockPolicyStore{counts: make(map[string]int64)}
}

func (m *mockPolicyStore) Record(_ context.Context, key string, _ time.Duration) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}

	m.counts[key]++

	return m.counts[key], nil
}

// mockScopeResolver is a mock resolver for testing.
type mockScopeResolver struct {
	scopes []ratelimit.Scope
}

func (m *mockScopeResolver) Resolve(_ huma.Context) []ratelimit.Scope {
	return m.scopes
}

//nolint:maintidx // Test function with comprehensive coverage across many scenarios
func TestPolicyRateLimiter(t *testing.T) {
	t.Run("allows request when under limit", func(t *testing.T) {
		api := newTestAPI()
		store := newMockPolicyStore()
		policy := ratelimit.NewPolicyBuilder().
			AddLimit(ratelimit.ScopeGlobal, 10, time.Minute).
			Build()
		limiter := ratelimit.NewPolicyLimiter(store, policy)
		resolver := &mockScopeResolver{scopes: []ratelimit.Scope{ratelimit.ScopeGlobal}}
		logger := zap.NewNop()

		mw := middleware.PolicyRateLimiter(api, limiter, resolver, logger)

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
		store := newMockPolicyStore()
		policy := ratelimit.NewPolicyBuilder().
			AddLimit(ratelimit.ScopeGlobal, 1, time.Minute).
			Build()
		limiter := ratelimit.NewPolicyLimiter(store, policy)
		resolver := &mockScopeResolver{scopes: []ratelimit.Scope{ratelimit.ScopeGlobal}}
		logger := zap.NewNop()

		mw := middleware.PolicyRateLimiter(api, limiter, resolver, logger)

		ctx := newMockHumaContext()
		ctx.host = testHostAddr
		ctx.headers["User-Agent"] = testUserAgent

		// First request allowed
		mw(ctx, func(_ huma.Context) {})

		// Second request should be denied
		ctx2 := newMockHumaContext()
		ctx2.host = testHostAddr
		ctx2.headers["User-Agent"] = testUserAgent

		nextCalled := false

		mw(ctx2, func(_ huma.Context) {
			nextCalled = true
		})

		assert.False(t, nextCalled, "next should not be called when rate limited")
		assert.Equal(t, 429, ctx2.statusCode)
		assert.Contains(t, string(ctx2.written), "rate limit exceeded")
	})

	t.Run("includes limit details in error message", func(t *testing.T) {
		api := newTestAPI()
		store := newMockPolicyStore()
		policy := ratelimit.NewPolicyBuilder().
			AddLimit(ratelimit.ScopeWrite, 1, time.Minute).
			Build()
		limiter := ratelimit.NewPolicyLimiter(store, policy)
		resolver := &mockScopeResolver{scopes: []ratelimit.Scope{ratelimit.ScopeWrite}}
		logger := zap.NewNop()

		mw := middleware.PolicyRateLimiter(api, limiter, resolver, logger)

		ctx := newMockHumaContext()
		ctx.host = testHostAddr
		ctx.headers["User-Agent"] = testUserAgent

		mw(ctx, func(_ huma.Context) {})

		ctx2 := newMockHumaContext()
		ctx2.host = testHostAddr
		ctx2.headers["User-Agent"] = testUserAgent

		mw(ctx2, func(_ huma.Context) {})

		assert.Contains(t, string(ctx2.written), "write")
		assert.Contains(t, string(ctx2.written), "2/1")
	})

	t.Run("applies different limits per scope", func(t *testing.T) {
		api := newTestAPI()
		store := newMockPolicyStore()
		policy := ratelimit.NewPolicyBuilder().
			AddLimit(ratelimit.ScopeRead, 5, time.Minute).
			AddLimit(ratelimit.ScopeWrite, 2, time.Minute).
			Build()
		limiter := ratelimit.NewPolicyLimiter(store, policy)
		logger := zap.NewNop()

		readResolver := &mockScopeResolver{scopes: []ratelimit.Scope{ratelimit.ScopeRead}}
		writeResolver := &mockScopeResolver{scopes: []ratelimit.Scope{ratelimit.ScopeWrite}}

		readMW := middleware.PolicyRateLimiter(api, limiter, readResolver, logger)
		writeMW := middleware.PolicyRateLimiter(api, limiter, writeResolver, logger)

		// Read requests - should allow 5
		for i := range 5 {
			ctx := newMockHumaContext()
			ctx.host = testHostAddr
			ctx.headers["User-Agent"] = testUserAgent

			nextCalled := false

			readMW(ctx, func(_ huma.Context) {
				nextCalled = true
			})

			assert.True(t, nextCalled, "read request %d should be allowed", i+1)
		}

		// Write requests - should only allow 2
		for i := range 2 {
			ctx := newMockHumaContext()
			ctx.host = testHostAddr
			ctx.headers["User-Agent"] = testUserAgent

			nextCalled := false

			writeMW(ctx, func(_ huma.Context) {
				nextCalled = true
			})

			assert.True(t, nextCalled, "write request %d should be allowed", i+1)
		}

		// 3rd write should be denied
		ctx := newMockHumaContext()
		ctx.host = testHostAddr
		ctx.headers["User-Agent"] = testUserAgent

		nextCalled := false

		writeMW(ctx, func(_ huma.Context) {
			nextCalled = true
		})

		assert.False(t, nextCalled, "3rd write request should be denied")
		assert.Equal(t, 429, ctx.statusCode)
	})

	t.Run("returns 500 on store error", func(t *testing.T) {
		api := newTestAPI()
		store := newMockPolicyStore()
		store.err = errors.New("store error")
		policy := ratelimit.NewPolicyBuilder().
			AddLimit(ratelimit.ScopeGlobal, 10, time.Minute).
			Build()
		limiter := ratelimit.NewPolicyLimiter(store, policy)
		resolver := &mockScopeResolver{scopes: []ratelimit.Scope{ratelimit.ScopeGlobal}}
		logger := zap.NewNop()

		mw := middleware.PolicyRateLimiter(api, limiter, resolver, logger)

		ctx := newMockHumaContext()
		ctx.host = testHostAddr
		ctx.headers["User-Agent"] = testUserAgent

		nextCalled := false

		mw(ctx, func(_ huma.Context) {
			nextCalled = true
		})

		assert.False(t, nextCalled)
		assert.Equal(t, 500, ctx.statusCode)
	})

	t.Run("skips rate limiting when disabled via metadata", func(t *testing.T) {
		api := newTestAPI()
		store := newMockPolicyStore()
		policy := ratelimit.NewPolicyBuilder().
			AddLimit(ratelimit.ScopeGlobal, 1, time.Minute).
			Build()
		limiter := ratelimit.NewPolicyLimiter(store, policy)
		resolver := &mockScopeResolver{scopes: []ratelimit.Scope{ratelimit.ScopeGlobal}}
		logger := zap.NewNop()

		mw := middleware.PolicyRateLimiter(api, limiter, resolver, logger)

		// First request with disabled rate limiting
		ctx := newMockHumaContext()
		ctx.host = testHostAddr
		ctx.headers["User-Agent"] = testUserAgent
		ctx.operation = &huma.Operation{
			Path: "/test",
			Metadata: map[string]any{
				ratelimit.MetadataKey: ratelimit.EndpointConfig{
					Disabled: true,
				},
			},
		}

		nextCalled := false

		mw(ctx, func(_ huma.Context) {
			nextCalled = true
		})

		assert.True(t, nextCalled, "next should be called when rate limiting is disabled")

		// Second request should also be allowed (disabled means no limit)
		ctx2 := newMockHumaContext()
		ctx2.host = testHostAddr
		ctx2.headers["User-Agent"] = testUserAgent
		ctx2.operation = ctx.operation

		nextCalled = false

		mw(ctx2, func(_ huma.Context) {
			nextCalled = true
		})

		assert.True(t, nextCalled, "second request should also be allowed when disabled")
	})

	t.Run("applies custom limits from metadata", func(t *testing.T) {
		api := newTestAPI()
		store := newMockPolicyStore()
		policy := ratelimit.NewPolicyBuilder().
			AddLimit(ratelimit.ScopeGlobal, 100, time.Minute). // Policy allows 100
			Build()
		limiter := ratelimit.NewPolicyLimiter(store, policy)
		resolver := &mockScopeResolver{scopes: []ratelimit.Scope{ratelimit.ScopeGlobal}}
		logger := zap.NewNop()

		mw := middleware.PolicyRateLimiter(api, limiter, resolver, logger)

		// Custom limit of 2 per minute
		operation := &huma.Operation{
			Path: "/custom",
			Metadata: map[string]any{
				ratelimit.MetadataKey: ratelimit.EndpointConfig{
					Limits: []ratelimit.LimitConfig{
						{Window: time.Minute, Max: 2},
					},
				},
			},
		}

		// First two requests should succeed
		for i := range 2 {
			ctx := newMockHumaContext()
			ctx.host = testHostAddr
			ctx.headers["User-Agent"] = testUserAgent
			ctx.operation = operation

			nextCalled := false

			mw(ctx, func(_ huma.Context) {
				nextCalled = true
			})

			assert.True(t, nextCalled, "request %d should be allowed", i+1)
		}

		// Third request should be rate limited
		ctx := newMockHumaContext()
		ctx.host = testHostAddr
		ctx.headers["User-Agent"] = testUserAgent
		ctx.operation = operation

		nextCalled := false

		mw(ctx, func(_ huma.Context) {
			nextCalled = true
		})

		assert.False(t, nextCalled, "third request should be denied by custom limit")
		assert.Equal(t, 429, ctx.statusCode)
	})

	t.Run("extracts path from operation", func(t *testing.T) {
		api := newTestAPI()
		store := newMockPolicyStore()
		policy := ratelimit.NewPolicyBuilder().
			AddLimit(ratelimit.ScopeGlobal, 10, time.Minute).
			Build()
		limiter := ratelimit.NewPolicyLimiter(store, policy)
		resolver := &mockScopeResolver{scopes: []ratelimit.Scope{ratelimit.ScopeGlobal}}
		logger := zap.NewNop()

		mw := middleware.PolicyRateLimiter(api, limiter, resolver, logger)

		ctx := newMockHumaContext()
		ctx.host = testHostAddr
		ctx.headers["User-Agent"] = testUserAgent
		ctx.operation = &huma.Operation{
			Path: "/api/v1/test",
		}

		nextCalled := false

		mw(ctx, func(_ huma.Context) {
			nextCalled = true
		})

		assert.True(t, nextCalled, "request should be allowed")
	})

	t.Run("custom limits store error returns 500", func(t *testing.T) {
		api := newTestAPI()
		store := newMockPolicyStore()
		store.err = errors.New("store error")
		policy := ratelimit.NewPolicyBuilder().Build()
		limiter := ratelimit.NewPolicyLimiter(store, policy)
		resolver := &mockScopeResolver{scopes: []ratelimit.Scope{}}
		logger := zap.NewNop()

		mw := middleware.PolicyRateLimiter(api, limiter, resolver, logger)

		ctx := newMockHumaContext()
		ctx.host = testHostAddr
		ctx.headers["User-Agent"] = testUserAgent
		ctx.operation = &huma.Operation{
			Path: "/custom-error",
			Metadata: map[string]any{
				ratelimit.MetadataKey: ratelimit.EndpointConfig{
					Limits: []ratelimit.LimitConfig{
						{Window: time.Minute, Max: 10},
					},
				},
			},
		}

		nextCalled := false

		mw(ctx, func(_ huma.Context) {
			nextCalled = true
		})

		assert.False(t, nextCalled)
		assert.Equal(t, 500, ctx.statusCode)
	})
}
