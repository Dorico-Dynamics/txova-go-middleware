// Package mwlog provides HTTP request logging middleware.
package mwlog

import (
	"bufio"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Dorico-Dynamics/txova-go-core/logging"

	middleware "github.com/Dorico-Dynamics/txova-go-middleware"
)

// Config holds configuration for the logging middleware.
type Config struct {
	// ExcludePaths are paths that should not be logged.
	ExcludePaths []string

	// MaskQueryParams are query parameter names whose values should be masked.
	MaskQueryParams []string

	// SlowRequestThreshold logs requests exceeding this duration at WARN level.
	// Zero disables slow request logging.
	SlowRequestThreshold time.Duration
}

// Option is a functional option for configuring the middleware.
type Option func(*Config)

// WithExcludePaths sets paths to exclude from logging.
func WithExcludePaths(paths ...string) Option {
	return func(c *Config) {
		c.ExcludePaths = paths
	}
}

// WithMaskQueryParams sets query parameter names to mask in logs.
func WithMaskQueryParams(params ...string) Option {
	return func(c *Config) {
		c.MaskQueryParams = params
	}
}

// WithSlowRequestThreshold sets the threshold for logging slow requests.
func WithSlowRequestThreshold(d time.Duration) Option {
	return func(c *Config) {
		c.SlowRequestThreshold = d
	}
}

// responseWriter wraps http.ResponseWriter to capture response metadata.
type responseWriter struct {
	http.ResponseWriter
	status       int
	bytesWritten int
	wroteHeader  bool
}

// WriteHeader captures the status code.
func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures bytes written and ensures WriteHeader is called.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// Flush implements http.Flusher.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements http.Hijacker.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Push implements http.Pusher.
func (rw *responseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := rw.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

// Middleware returns an HTTP middleware that logs requests.
func Middleware(logger *logging.Logger, opts ...Option) func(http.Handler) http.Handler {
	cfg := Config{
		ExcludePaths:    []string{"/health", "/ready", "/healthz", "/readyz"},
		MaskQueryParams: []string{"token", "key", "secret", "password", "api_key", "apikey"},
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	excludeSet := make(map[string]bool, len(cfg.ExcludePaths))
	for _, p := range cfg.ExcludePaths {
		excludeSet[p] = true
	}

	maskSet := make(map[string]bool, len(cfg.MaskQueryParams))
	for _, p := range cfg.MaskQueryParams {
		maskSet[strings.ToLower(p)] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip excluded paths.
			if excludeSet[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Wrap response writer to capture status and bytes.
			rw := &responseWriter{
				ResponseWriter: w,
				status:         http.StatusOK,
			}

			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			// Extract context values.
			requestID := middleware.RequestIDFromContext(r.Context())
			userID, _ := middleware.UserIDFromContext(r.Context())

			// Build log attributes.
			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
				"duration_ms", duration.Milliseconds(),
				"request_id", requestID,
				"user_id", userID,
				"ip", middleware.GetClientIP(r),
				"user_agent", r.UserAgent(),
				"bytes_in", r.ContentLength,
				"bytes_out", rw.bytesWritten,
			}

			// Add masked query params if present.
			if r.URL.RawQuery != "" {
				attrs = append(attrs, "query", maskQueryParams(r.URL.RawQuery, maskSet))
			}

			// Determine log level based on status code.
			logCtx := logger.WithContext(r.Context())
			switch {
			case cfg.SlowRequestThreshold > 0 && duration > cfg.SlowRequestThreshold:
				attrs = append(attrs, "slow", true)
				logCtx.Warn("http request", attrs...)
			case rw.status >= 500:
				logCtx.Error("http request", attrs...)
			case rw.status >= 400:
				logCtx.Warn("http request", attrs...)
			default:
				logCtx.Info("http request", attrs...)
			}
		})
	}
}

// maskQueryParams masks sensitive query parameter values.
func maskQueryParams(rawQuery string, maskSet map[string]bool) string {
	if rawQuery == "" {
		return ""
	}

	parts := strings.Split(rawQuery, "&")
	for i, part := range parts {
		if idx := strings.Index(part, "="); idx > 0 {
			key := strings.ToLower(part[:idx])
			if maskSet[key] {
				parts[i] = part[:idx+1] + "[REDACTED]"
			}
		}
	}
	return strings.Join(parts, "&")
}
