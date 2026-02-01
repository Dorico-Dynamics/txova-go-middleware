// Package requestid provides middleware for generating and propagating request IDs.
package requestid

import (
	"net/http"

	"github.com/google/uuid"

	middleware "github.com/Dorico-Dynamics/txova-go-middleware"
)

// DefaultHeaderName is the default HTTP header name for request IDs.
const DefaultHeaderName = "X-Request-ID"

// Generator is a function type for generating request IDs.
type Generator func() string

// DefaultGenerator generates a UUID v4 request ID.
func DefaultGenerator() string {
	return uuid.New().String()
}

// Config holds configuration for the request ID middleware.
type Config struct {
	// HeaderName is the HTTP header name to read/write the request ID.
	// Defaults to "X-Request-ID".
	HeaderName string

	// Generator is the function to generate new request IDs.
	// Defaults to UUID v4 generation.
	Generator Generator
}

// Option is a functional option for configuring the middleware.
type Option func(*Config)

// WithHeaderName sets a custom header name for the request ID.
func WithHeaderName(name string) Option {
	return func(c *Config) {
		c.HeaderName = name
	}
}

// WithGenerator sets a custom request ID generator.
func WithGenerator(g Generator) Option {
	return func(c *Config) {
		c.Generator = g
	}
}

// Middleware returns an HTTP middleware that injects request IDs.
// If the request contains an existing request ID header, it will be preserved.
// Otherwise, a new UUID v4 request ID is generated.
// The request ID is injected into the request context and added to the response headers.
func Middleware(opts ...Option) func(http.Handler) http.Handler {
	cfg := Config{
		HeaderName: DefaultHeaderName,
		Generator:  DefaultGenerator,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get(cfg.HeaderName)

			if requestID == "" {
				requestID = cfg.Generator()
			}

			// Inject into context.
			ctx := middleware.WithRequestID(r.Context(), requestID)

			// Add to response headers.
			w.Header().Set(cfg.HeaderName, requestID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
