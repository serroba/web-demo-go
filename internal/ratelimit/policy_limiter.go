package ratelimit

import (
	"context"
	"fmt"
)

// LimitExceeded contains information about which limit was exceeded.
type LimitExceeded struct {
	Scope  Scope
	Config LimitConfig
	Count  int64
}

// PolicyLimiter enforces rate limits based on a policy and resolved scopes.
type PolicyLimiter struct {
	store  Store
	policy *Policy
}

// NewPolicyLimiter creates a new policy-based rate limiter.
func NewPolicyLimiter(store Store, policy *Policy) *PolicyLimiter {
	return &PolicyLimiter{
		store:  store,
		policy: policy,
	}
}

// Allow checks if a request should be allowed based on the client key and applicable scopes.
// It returns true if the request is allowed, false if any limit is exceeded.
// The LimitExceeded return value provides details about which limit was hit (nil if allowed).
func (l *PolicyLimiter) Allow(ctx context.Context, clientKey string, scopes []Scope) (bool, *LimitExceeded, error) {
	for _, scope := range scopes {
		limits, ok := l.policy.Limits[scope]
		if !ok {
			continue
		}

		for _, limit := range limits {
			// Key combines client + scope + window for independent tracking
			key := l.buildKey(clientKey, scope, limit)

			count, err := l.store.Record(ctx, key, limit.Window)
			if err != nil {
				return false, nil, err
			}

			if count > limit.Max {
				return false, &LimitExceeded{
					Scope:  scope,
					Config: limit,
					Count:  count,
				}, nil
			}
		}
	}

	return true, nil, nil
}

// buildKey creates a unique rate limit key for the client, scope, and window combination.
func (l *PolicyLimiter) buildKey(clientKey string, scope Scope, limit LimitConfig) string {
	return fmt.Sprintf("%s:%s:%d", clientKey, scope, limit.Window.Milliseconds())
}

// Store returns the underlying rate limit store.
func (l *PolicyLimiter) Store() Store {
	return l.store
}
