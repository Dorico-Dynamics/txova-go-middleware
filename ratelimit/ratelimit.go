// Package ratelimit provides rate limiting middleware for the Txova platform.
package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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

// RedisClientWithEval extends RedisClient with Lua script support for atomic operations.
// If the client implements this interface, atomic INCR+EXPIRE will be used.
type RedisClientWithEval interface {
	RedisClient
	// Eval executes a Lua script with the given keys and args.
	Eval(ctx context.Context, script string, keys []string, args ...any) (any, error)
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

// incrWithExpireScript is a Lua script that atomically increments a key and sets expiration.
// Returns the new count. Sets expiration only on first increment (when count becomes 1).
const incrWithExpireScript = `
local count = redis.call('INCR', KEYS[1])
if count == 1 then
    redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return count
`

// Check checks if a request should be rate limited.
// Returns the current count, remaining requests, reset time, and whether the request is allowed.
func (l *Limiter) Check(ctx context.Context, key string) (count int64, remaining int, resetAt time.Time, allowed bool) {
	fullKey := fmt.Sprintf("%s:%s", l.keyPrefix, key)

	// Try to use atomic INCR+EXPIRE if client supports Eval.
	if evalClient, ok := l.client.(RedisClientWithEval); ok {
		atomicCount, err := l.checkAtomic(ctx, evalClient, fullKey)
		if err != nil {
			// On error, allow the request to avoid blocking traffic.
			return 0, l.limit, time.Now().Add(l.window), true
		}
		return l.calculateResult(ctx, fullKey, atomicCount)
	}

	// Fallback to non-atomic approach for clients without Eval support.
	count, err := l.client.Incr(ctx, fullKey)
	if err != nil {
		// On error, allow the request to avoid blocking traffic.
		return 0, l.limit, time.Now().Add(l.window), true
	}

	// Set expiration on first request.
	// Note: This is non-atomic and may leave keys without TTL in edge cases.
	if count == 1 {
		// Ignore expiration errors - the key will still work, just may persist longer.
		//nolint:errcheck // Intentionally ignoring - see comment above
		_ = l.client.Expire(ctx, fullKey, l.window)
	}

	return l.calculateResult(ctx, fullKey, count)
}

// checkAtomic performs an atomic increment with expiration using a Lua script.
func (l *Limiter) checkAtomic(ctx context.Context, client RedisClientWithEval, fullKey string) (int64, error) {
	windowSeconds := int64(l.window.Seconds())
	result, err := client.Eval(ctx, incrWithExpireScript, []string{fullKey}, windowSeconds)
	if err != nil {
		return 0, err
	}

	// Handle different return types from Redis.
	switch v := result.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("unexpected result type from Lua script: %T", result)
	}
}

// calculateResult computes the remaining requests and reset time.
func (l *Limiter) calculateResult(ctx context.Context, fullKey string, count int64) (int64, int, time.Time, bool) {
	// Get TTL for reset time.
	ttl, err := l.client.TTL(ctx, fullKey)
	if err != nil || ttl <= 0 {
		ttl = l.window
	}
	resetAt := time.Now().Add(ttl)

	// Calculate remaining.
	remaining := l.limit - int(count)
	if remaining < 0 {
		remaining = 0
	}

	allowed := int(count) <= l.limit

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

	// Compute Retry-After, ensuring it's never negative.
	retrySec := int64(time.Until(resetAt).Seconds())
	if retrySec < 0 {
		retrySec = 0
	}
	w.Header().Set("Retry-After", strconv.FormatInt(retrySec, 10))

	err := errors.RateLimited("rate limit exceeded")
	w.WriteHeader(err.HTTPStatus())

	// Attempt to write JSON response. If encoding fails, write plain text fallback.
	// Do not call http.Error as it would invoke WriteHeader again.
	if encodeErr := json.NewEncoder(w).Encode(err.ToResponse()); encodeErr != nil {
		// WriteHeader already called, just write body. Ignore write error as
		// there's nothing we can do if writing the fallback also fails.
		//nolint:errcheck // Intentional - fallback write, nothing to do on error
		_, _ = w.Write([]byte(http.StatusText(http.StatusTooManyRequests)))
	}
}

// Built-in key functions.

// KeyByIP returns a key function that rate limits by client IP address.
func KeyByIP(r *http.Request) string {
	return "ip:" + middleware.GetClientIP(r)
}

// KeyByUser returns a key function that rate limits by authenticated user ID.
func KeyByUser(r *http.Request) string {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		// Fall back to IP if user is not authenticated.
		return "ip:" + middleware.GetClientIP(r)
	}
	return "user:" + userID
}

// KeyByEndpoint returns a key function that rate limits by request path.
func KeyByEndpoint(r *http.Request) string {
	return "path:" + r.URL.Path
}

// KeyByIPAndEndpoint returns a key function that rate limits by IP and path.
func KeyByIPAndEndpoint(r *http.Request) string {
	return "ip:" + middleware.GetClientIP(r) + ":path:" + r.URL.Path
}

// KeyByUserAndEndpoint returns a key function that rate limits by user and path.
func KeyByUserAndEndpoint(r *http.Request) string {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		return "ip:" + middleware.GetClientIP(r) + ":path:" + r.URL.Path
	}
	return "user:" + userID + ":path:" + r.URL.Path
}
