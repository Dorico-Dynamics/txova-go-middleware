// Package mwcors provides CORS middleware for the Txova platform.
package mwcors

import (
	"net/http"

	"github.com/go-chi/cors"
)

// Config holds configuration for the CORS middleware.
type Config struct {
	// AllowedOrigins is a list of origins a cross-domain request can be executed from.
	// Use "*" to allow all origins (only for development).
	AllowedOrigins []string

	// AllowedMethods is a list of methods the client is allowed to use.
	AllowedMethods []string

	// AllowedHeaders is a list of headers the client is allowed to use.
	AllowedHeaders []string

	// ExposedHeaders is a list of headers that are safe to expose to the client.
	ExposedHeaders []string

	// MaxAge indicates how long (in seconds) the results of a preflight request can be cached.
	MaxAge int

	// AllowCredentials indicates whether the request can include user credentials.
	AllowCredentials bool
}

// DefaultConfig returns a sensible default CORS configuration.
// Note: AllowedOrigins is empty by default, which rejects all cross-origin requests.
// You must explicitly set allowed origins for your application.
func DefaultConfig() Config {
	return Config{
		AllowedOrigins:   []string{},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		MaxAge:           86400, // 24 hours
		AllowCredentials: true,
	}
}

// DevelopmentConfig returns a permissive CORS configuration for development.
// WARNING: Do not use in production as it allows all origins.
func DevelopmentConfig() Config {
	cfg := DefaultConfig()
	cfg.AllowedOrigins = []string{"*"}
	return cfg
}

// Option is a functional option for configuring the middleware.
type Option func(*Config)

// WithAllowedOrigins sets the allowed origins.
func WithAllowedOrigins(origins ...string) Option {
	return func(c *Config) {
		c.AllowedOrigins = origins
	}
}

// WithAllowedMethods sets the allowed HTTP methods.
func WithAllowedMethods(methods ...string) Option {
	return func(c *Config) {
		c.AllowedMethods = methods
	}
}

// WithAllowedHeaders sets the allowed request headers.
func WithAllowedHeaders(headers ...string) Option {
	return func(c *Config) {
		c.AllowedHeaders = headers
	}
}

// WithExposedHeaders sets the headers exposed to the client.
func WithExposedHeaders(headers ...string) Option {
	return func(c *Config) {
		c.ExposedHeaders = headers
	}
}

// WithMaxAge sets the preflight cache duration in seconds.
func WithMaxAge(seconds int) Option {
	return func(c *Config) {
		c.MaxAge = seconds
	}
}

// WithAllowCredentials enables or disables credentials support.
func WithAllowCredentials(allow bool) Option {
	return func(c *Config) {
		c.AllowCredentials = allow
	}
}

// Middleware returns an HTTP middleware that handles CORS.
func Middleware(opts ...Option) func(http.Handler) http.Handler {
	cfg := DefaultConfig()

	for _, opt := range opts {
		opt(&cfg)
	}

	return cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   cfg.AllowedMethods,
		AllowedHeaders:   cfg.AllowedHeaders,
		ExposedHeaders:   cfg.ExposedHeaders,
		MaxAge:           cfg.MaxAge,
		AllowCredentials: cfg.AllowCredentials,
	})
}

// MiddlewareWithConfig returns an HTTP middleware with explicit configuration.
func MiddlewareWithConfig(cfg Config) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   cfg.AllowedMethods,
		AllowedHeaders:   cfg.AllowedHeaders,
		ExposedHeaders:   cfg.ExposedHeaders,
		MaxAge:           cfg.MaxAge,
		AllowCredentials: cfg.AllowCredentials,
	})
}
