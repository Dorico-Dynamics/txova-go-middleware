// Package ratelimit provides rate limiting middleware for the Txova platform.
package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Dorico-Dynamics/txova-go-core/errors"

	middleware "github.com/Dorico-Dynamics/txova-go-middleware"
)

// RedisClient defines the interface for Redis operations required by the rate limiter.
// This interface is compatible with go-redis/redis.
type RedisClient interface {
	// Incr increments a key's value and returns the new value.
	Incr(ctx context.Context, key string) (int64, error)
	// Expire sets a key's expiration time.
	Expire(ctx context.Context, key string, expiration time.Duration) error
	// TTL returns the remaining time-to-live of a key.
	TTL(ctx context.Context, key string) (time.Duration, error)
}

// KeyFunc is a function type that extracts a rate limit key from a request.
type KeyFunc func(r *http.Request) string

// SkipFunc is a function type that determines if rate limiting should be skipped.
type SkipFunc func(r *http.Request) bool

// Config holds configuration for the rate limiter.
type Config struct {
	// Limit is the maximum number of requests per window.
	Limit int

	// Window is the time window for rate limiting.
	Window time.Duration

	// BurstAllowance is extra requests allowed in a burst (added to Limit).
	BurstAllowance int

	// KeyFunc extracts the rate limit key from requests.
	KeyFunc KeyFunc

	// SkipFunc determines if rate limiting should be skipped.
	SkipFunc SkipFunc

	// KeyPrefix is the prefix for Redis keys (default: "ratelimit").
	KeyPrefix string
}

// Option is a functional option for configuring the rate limiter.
type Option func(*Config)

// WithLimit sets the request limit per window.
func WithLimit(limit int) Option {
	return func(c *Config) {
		c.Limit = limit
	}
}

// WithWindow sets the time window for rate limiting.
func WithWindow(window time.Duration) Option {
	return func(c *Config) {
		c.Window = window
	}
}

// WithBurstAllowance sets the burst allowance.
func WithBurstAllowance(burst int) Option {
	return func(c *Config) {
		c.BurstAllowance = burst
	}
}

// WithKeyFunc sets a custom key extraction function.
func WithKeyFunc(fn KeyFunc) Option {
	return func(c *Config) {
		c.KeyFunc = fn
	}
}

// WithSkipFunc sets a function to skip rate limiting for certain requests.
func WithSkipFunc(fn SkipFunc) Option {
	return func(c *Config) {
		c.SkipFunc = fn
	}
}

// WithKeyPrefix sets the Redis key prefix.
func WithKeyPrefix(prefix string) Option {
	return func(c *Config) {
		c.KeyPrefix = prefix
	}
}

// Limiter implements rate limiting using a fixed window counter algorithm.
type Limiter struct {
	client    RedisClient
	limit     int
	window    time.Duration
	keyPrefix string
}

// NewLimiter creates a new rate limiter with the given Redis client and options.
func NewLimiter(client RedisClient, opts ...Option) *Limiter {
	cfg := Config{
		Limit:     100,
		Window:    time.Minute,
		KeyPrefix: "ratelimit",
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	// Add burst allowance to limit.
	effectiveLimit := cfg.Limit + cfg.BurstAllowance

	return &Limiter{
		client:    client,
		limit:     effectiveLimit,
		window:    cfg.Window,
		keyPrefix: cfg.KeyPrefix,
	}
}

// Check checks if a request should be rate limited.
// Returns the current count, remaining requests, reset time, and whether the request is allowed.
func (l *Limiter) Check(ctx context.Context, key string) (count int64, remaining int, resetAt time.Time, allowed bool) {
	fullKey := fmt.Sprintf("%s:%s", l.keyPrefix, key)

	// Increment the counter.
	count, err := l.client.Incr(ctx, fullKey)
	if err != nil {
		// On error, allow the request to avoid blocking traffic.
		return 0, l.limit, time.Now().Add(l.window), true
	}

	// Set expiration on first request.
	if count == 1 {
		// Ignore expiration errors - the key will still work, just may persist longer.
		expireErr := l.client.Expire(ctx, fullKey, l.window)
		_ = expireErr
	}

	// Get TTL for reset time.
	ttl, err := l.client.TTL(ctx, fullKey)
	if err != nil || ttl <= 0 {
		ttl = l.window
	}
	resetAt = time.Now().Add(ttl)

	// Calculate remaining.
	remaining = l.limit - int(count)
	if remaining < 0 {
		remaining = 0
	}

	allowed = int(count) <= l.limit

	return count, remaining, resetAt, allowed
}

// Middleware returns an HTTP middleware that enforces rate limiting.
func Middleware(limiter *Limiter, opts ...Option) func(http.Handler) http.Handler {
	cfg := Config{
		KeyFunc: KeyByIP,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	keyFunc := cfg.KeyFunc
	if keyFunc == nil {
		keyFunc = KeyByIP
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if we should skip rate limiting.
			if cfg.SkipFunc != nil && cfg.SkipFunc(r) {
				next.ServeHTTP(w, r)
				return
			}

			// Get the rate limit key.
			key := keyFunc(r)

			// Check rate limit.
			_, remaining, resetAt, allowed := limiter.Check(r.Context(), key)

			// Set rate limit headers.
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limiter.limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))

			if !allowed {
				writeRateLimitError(w, resetAt)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// writeRateLimitError writes a rate limit exceeded response.
func writeRateLimitError(w http.ResponseWriter, resetAt time.Time) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Retry-After", strconv.FormatInt(int64(time.Until(resetAt).Seconds()), 10))

	err := errors.RateLimited("rate limit exceeded")
	w.WriteHeader(err.HTTPStatus())
	if encodeErr := json.NewEncoder(w).Encode(err.ToResponse()); encodeErr != nil {
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
	}
}

// Built-in key functions.

// KeyByIP returns a key function that rate limits by client IP address.
func KeyByIP(r *http.Request) string {
	return "ip:" + getClientIP(r)
}

// KeyByUser returns a key function that rate limits by authenticated user ID.
func KeyByUser(r *http.Request) string {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		// Fall back to IP if user is not authenticated.
		return "ip:" + getClientIP(r)
	}
	return "user:" + userID
}

// KeyByEndpoint returns a key function that rate limits by request path.
func KeyByEndpoint(r *http.Request) string {
	return "path:" + r.URL.Path
}

// KeyByIPAndEndpoint returns a key function that rate limits by IP and path.
func KeyByIPAndEndpoint(r *http.Request) string {
	return "ip:" + getClientIP(r) + ":path:" + r.URL.Path
}

// KeyByUserAndEndpoint returns a key function that rate limits by user and path.
func KeyByUserAndEndpoint(r *http.Request) string {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		return "ip:" + getClientIP(r) + ":path:" + r.URL.Path
	}
	return "user:" + userID + ":path:" + r.URL.Path
}

// getClientIP extracts the client IP address from the request.
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Remove port from RemoteAddr.
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx > 0 {
		return addr[:idx]
	}

	return addr
}
