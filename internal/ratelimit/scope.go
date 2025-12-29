package ratelimit

import "github.com/danielgtaylor/huma/v2"

// Scope categorizes a request for rate limiting purposes.
// Different scopes can have different rate limits applied.
type Scope string

const (
	// ScopeGlobal applies to all requests regardless of type.
	ScopeGlobal Scope = "global"
	// ScopeRead applies to read operations (GET, HEAD, OPTIONS).
	ScopeRead Scope = "read"
	// ScopeWrite applies to write operations (POST, PUT, PATCH, DELETE).
	ScopeWrite Scope = "write"
)

// MetadataKey is the key used to store rate limit config in operation metadata.
const MetadataKey = "rateLimit"

// EndpointConfig defines per-endpoint rate limit configuration.
// This can be attached to Huma operations via the Metadata field.
type EndpointConfig struct {
	// Scope overrides the default scope detection (read/write based on method)
	// when no custom Limits are configured for the endpoint.
	//
	// If Scope is empty and Limits is nil or empty, the middleware falls back
	// to method-based scope detection (read/write based on HTTP method).
	//
	// NOTE: When custom Limits are provided, the middleware applies those
	// limits directly and does not use scope-based default limits, so Scope
	// has no effect in that case.
	Scope Scope

	// Limits defines custom rate limits for this endpoint.
	//
	// If Limits is nil or empty, the default policy limits for the resolved
	// scopes are used (based on Scope, or method-based detection when Scope
	// is empty).
	//
	// When Limits is non-empty, the middleware bypasses scope-based default
	// limits and uses only the provided Limits; Scope is ignored.
	Limits []LimitConfig

	// Disabled skips rate limiting entirely for this endpoint.
	Disabled bool
}

// ScopeResolver determines which scopes apply to a given request.
type ScopeResolver interface {
	Resolve(ctx huma.Context) []Scope
}

// MethodScopeResolver resolves scopes based on HTTP method.
// GET, HEAD, OPTIONS are classified as read operations.
// All other methods are classified as write operations.
type MethodScopeResolver struct{}

// NewMethodScopeResolver creates a new method-based scope resolver.
func NewMethodScopeResolver() *MethodScopeResolver {
	return &MethodScopeResolver{}
}

// Resolve returns the scopes that apply to the request based on its HTTP method.
func (r *MethodScopeResolver) Resolve(ctx huma.Context) []Scope {
	scopes := []Scope{ScopeGlobal}

	switch ctx.Method() {
	case "GET", "HEAD", "OPTIONS":
		scopes = append(scopes, ScopeRead)
	default:
		scopes = append(scopes, ScopeWrite)
	}

	return scopes
}

// OperationScopeResolver resolves scopes by checking operation metadata first,
// then falling back to method-based detection.
type OperationScopeResolver struct {
	fallback *MethodScopeResolver
}

// NewOperationScopeResolver creates a new operation-aware scope resolver.
func NewOperationScopeResolver() *OperationScopeResolver {
	return &OperationScopeResolver{
		fallback: NewMethodScopeResolver(),
	}
}

// Resolve returns the scopes for a request, checking operation metadata first.
func (r *OperationScopeResolver) Resolve(ctx huma.Context) []Scope {
	op := ctx.Operation()
	if op == nil || op.Metadata == nil {
		return r.fallback.Resolve(ctx)
	}

	cfg, ok := op.Metadata[MetadataKey].(EndpointConfig)
	if !ok {
		return r.fallback.Resolve(ctx)
	}

	// If a specific scope is configured, use it
	if cfg.Scope != "" {
		return []Scope{ScopeGlobal, cfg.Scope}
	}

	return r.fallback.Resolve(ctx)
}

// GetEndpointConfig extracts the EndpointConfig from operation metadata, if present.
func GetEndpointConfig(ctx huma.Context) *EndpointConfig {
	op := ctx.Operation()
	if op == nil || op.Metadata == nil {
		return nil
	}

	cfg, ok := op.Metadata[MetadataKey].(EndpointConfig)
	if !ok {
		return nil
	}

	return &cfg
}
