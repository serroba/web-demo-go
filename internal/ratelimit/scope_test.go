package ratelimit_test

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
	"github.com/serroba/web-demo-go/internal/ratelimit"
	"github.com/stretchr/testify/assert"
)

var errMultipartNotSupported = errors.New("multipart not supported in mock")

// mockHumaContext implements huma.Context for testing scope resolution.
type mockHumaContext struct {
	method    string
	operation *huma.Operation
}

func (m *mockHumaContext) Operation() *huma.Operation {
	return m.operation
}
func (m *mockHumaContext) Context() context.Context          { return context.Background() }
func (m *mockHumaContext) TLS() *tls.ConnectionState         { return nil }
func (m *mockHumaContext) Version() huma.ProtoVersion        { return huma.ProtoVersion{} }
func (m *mockHumaContext) Method() string                    { return m.method }
func (m *mockHumaContext) Host() string                      { return "" }
func (m *mockHumaContext) RemoteAddr() string                { return "" }
func (m *mockHumaContext) URL() url.URL                      { return url.URL{} }
func (m *mockHumaContext) Param(_ string) string             { return "" }
func (m *mockHumaContext) Query(_ string) string             { return "" }
func (m *mockHumaContext) Header(_ string) string            { return "" }
func (m *mockHumaContext) EachHeader(_ func(string, string)) {}
func (m *mockHumaContext) BodyReader() io.Reader             { return nil }
func (m *mockHumaContext) GetMultipartForm() (*multipart.Form, error) {
	return nil, errMultipartNotSupported
}
func (m *mockHumaContext) SetReadDeadline(_ time.Time) error { return nil }
func (m *mockHumaContext) SetStatus(_ int)                   {}
func (m *mockHumaContext) Status() int                       { return 0 }
func (m *mockHumaContext) AppendHeader(_, _ string)          {}
func (m *mockHumaContext) SetHeader(_, _ string)             {}
func (m *mockHumaContext) BodyWriter() io.Writer             { return nil }

func TestMethodScopeResolver_Resolve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		method         string
		expectedScopes []ratelimit.Scope
	}{
		{
			name:           "GET is classified as read",
			method:         "GET",
			expectedScopes: []ratelimit.Scope{ratelimit.ScopeGlobal, ratelimit.ScopeRead},
		},
		{
			name:           "HEAD is classified as read",
			method:         "HEAD",
			expectedScopes: []ratelimit.Scope{ratelimit.ScopeGlobal, ratelimit.ScopeRead},
		},
		{
			name:           "OPTIONS is classified as read",
			method:         "OPTIONS",
			expectedScopes: []ratelimit.Scope{ratelimit.ScopeGlobal, ratelimit.ScopeRead},
		},
		{
			name:           "POST is classified as write",
			method:         "POST",
			expectedScopes: []ratelimit.Scope{ratelimit.ScopeGlobal, ratelimit.ScopeWrite},
		},
		{
			name:           "PUT is classified as write",
			method:         "PUT",
			expectedScopes: []ratelimit.Scope{ratelimit.ScopeGlobal, ratelimit.ScopeWrite},
		},
		{
			name:           "PATCH is classified as write",
			method:         "PATCH",
			expectedScopes: []ratelimit.Scope{ratelimit.ScopeGlobal, ratelimit.ScopeWrite},
		},
		{
			name:           "DELETE is classified as write",
			method:         "DELETE",
			expectedScopes: []ratelimit.Scope{ratelimit.ScopeGlobal, ratelimit.ScopeWrite},
		},
	}

	resolver := ratelimit.NewMethodScopeResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := &mockHumaContext{method: tt.method}
			scopes := resolver.Resolve(ctx)

			assert.Equal(t, tt.expectedScopes, scopes)
		})
	}
}

func TestMethodScopeResolver_AlwaysIncludesGlobal(t *testing.T) {
	t.Parallel()

	resolver := ratelimit.NewMethodScopeResolver()

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		ctx := &mockHumaContext{method: method}
		scopes := resolver.Resolve(ctx)

		assert.Contains(t, scopes, ratelimit.ScopeGlobal, "method %s should include global scope", method)
	}
}

func TestOperationScopeResolver_FallsBackToMethodResolver(t *testing.T) {
	t.Parallel()

	resolver := ratelimit.NewOperationScopeResolver()

	tests := []struct {
		name           string
		method         string
		operation      *huma.Operation
		expectedScopes []ratelimit.Scope
	}{
		{
			name:           "nil operation falls back to method resolver",
			method:         "GET",
			operation:      nil,
			expectedScopes: []ratelimit.Scope{ratelimit.ScopeGlobal, ratelimit.ScopeRead},
		},
		{
			name:           "operation without metadata falls back to method resolver",
			method:         "POST",
			operation:      &huma.Operation{},
			expectedScopes: []ratelimit.Scope{ratelimit.ScopeGlobal, ratelimit.ScopeWrite},
		},
		{
			name:   "operation with unrelated metadata falls back to method resolver",
			method: "GET",
			operation: &huma.Operation{
				Metadata: map[string]any{"other": "value"},
			},
			expectedScopes: []ratelimit.Scope{ratelimit.ScopeGlobal, ratelimit.ScopeRead},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := &mockHumaContext{method: tt.method, operation: tt.operation}
			scopes := resolver.Resolve(ctx)

			assert.Equal(t, tt.expectedScopes, scopes)
		})
	}
}

func TestOperationScopeResolver_UsesMetadataScope(t *testing.T) {
	t.Parallel()

	resolver := ratelimit.NewOperationScopeResolver()

	tests := []struct {
		name           string
		method         string
		configScope    ratelimit.Scope
		expectedScopes []ratelimit.Scope
	}{
		{
			name:           "overrides GET to write scope",
			method:         "GET",
			configScope:    ratelimit.ScopeWrite,
			expectedScopes: []ratelimit.Scope{ratelimit.ScopeGlobal, ratelimit.ScopeWrite},
		},
		{
			name:           "overrides POST to read scope",
			method:         "POST",
			configScope:    ratelimit.ScopeRead,
			expectedScopes: []ratelimit.Scope{ratelimit.ScopeGlobal, ratelimit.ScopeRead},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := &mockHumaContext{
				method: tt.method,
				operation: &huma.Operation{
					Metadata: map[string]any{
						ratelimit.MetadataKey: ratelimit.EndpointConfig{
							Scope: tt.configScope,
						},
					},
				},
			}
			scopes := resolver.Resolve(ctx)

			assert.Equal(t, tt.expectedScopes, scopes)
		})
	}
}

func TestOperationScopeResolver_EmptyScopeFallsBack(t *testing.T) {
	t.Parallel()

	resolver := ratelimit.NewOperationScopeResolver()

	// EndpointConfig with empty scope should fall back to method-based detection
	ctx := &mockHumaContext{
		method: "POST",
		operation: &huma.Operation{
			Metadata: map[string]any{
				ratelimit.MetadataKey: ratelimit.EndpointConfig{
					// Scope is empty, should fall back
					Limits: []ratelimit.LimitConfig{
						{Window: time.Minute, Max: 10},
					},
				},
			},
		},
	}

	scopes := resolver.Resolve(ctx)

	// Should use method-based detection (POST = write)
	assert.Equal(t, []ratelimit.Scope{ratelimit.ScopeGlobal, ratelimit.ScopeWrite}, scopes)
}

func TestGetEndpointConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		operation *huma.Operation
		wantNil   bool
	}{
		{
			name:      "nil operation returns nil",
			operation: nil,
			wantNil:   true,
		},
		{
			name:      "operation without metadata returns nil",
			operation: &huma.Operation{},
			wantNil:   true,
		},
		{
			name: "operation with wrong type returns nil",
			operation: &huma.Operation{
				Metadata: map[string]any{
					ratelimit.MetadataKey: "wrong type",
				},
			},
			wantNil: true,
		},
		{
			name: "operation with valid config returns config",
			operation: &huma.Operation{
				Metadata: map[string]any{
					ratelimit.MetadataKey: ratelimit.EndpointConfig{
						Scope:    ratelimit.ScopeRead,
						Disabled: true,
					},
				},
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := &mockHumaContext{operation: tt.operation}
			cfg := ratelimit.GetEndpointConfig(ctx)

			if tt.wantNil {
				assert.Nil(t, cfg)
			} else {
				assert.NotNil(t, cfg)
				assert.Equal(t, ratelimit.ScopeRead, cfg.Scope)
				assert.True(t, cfg.Disabled)
			}
		})
	}
}
