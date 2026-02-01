// Package timeout provides middleware for enforcing request timeouts.
package timeout

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/Dorico-Dynamics/txova-go-core/errors"

	middleware "github.com/Dorico-Dynamics/txova-go-middleware"
)

// DefaultTimeout is the default request timeout.
const DefaultTimeout = 30 * time.Second

// Config holds configuration for the timeout middleware.
type Config struct {
	// Timeout is the maximum duration for a request.
	Timeout time.Duration

	// SkipPaths are paths that should bypass the timeout.
	SkipPaths []string
}

// Option is a functional option for configuring the middleware.
type Option func(*Config)

// WithTimeout sets the request timeout duration.
func WithTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.Timeout = d
	}
}

// WithSkipPaths sets paths that bypass the timeout middleware.
func WithSkipPaths(paths ...string) Option {
	return func(c *Config) {
		c.SkipPaths = paths
	}
}

// timeoutWriter wraps http.ResponseWriter to prevent writes after timeout.
type timeoutWriter struct {
	http.ResponseWriter
	mu          sync.Mutex
	timedOut    bool
	wroteHeader bool
	code        int
}

// WriteHeader captures the status code and prevents writes after timeout.
func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if tw.timedOut || tw.wroteHeader {
		return
	}

	tw.wroteHeader = true
	tw.code = code
	tw.ResponseWriter.WriteHeader(code)
}

// Write writes data and prevents writes after timeout.
func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if tw.timedOut {
		return 0, context.DeadlineExceeded
	}

	if !tw.wroteHeader {
		tw.wroteHeader = true
		tw.code = http.StatusOK
		tw.ResponseWriter.WriteHeader(http.StatusOK)
	}

	return tw.ResponseWriter.Write(b)
}

// setTimedOut marks the writer as timed out.
func (tw *timeoutWriter) setTimedOut() {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.timedOut = true
}

// hasWritten returns whether a response has been written.
func (tw *timeoutWriter) hasWritten() bool {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	return tw.wroteHeader
}

// Middleware returns an HTTP middleware that enforces request timeouts.
// If the handler does not complete within the timeout, a 503 Service Unavailable
// response is returned. The handler's context is cancelled on timeout.
func Middleware(opts ...Option) func(http.Handler) http.Handler {
	cfg := Config{
		Timeout: DefaultTimeout,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	skipSet := make(map[string]bool, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skipSet[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip timeout for configured paths.
			if skipSet[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Create context with timeout.
			ctx, cancel := context.WithTimeout(r.Context(), cfg.Timeout)
			defer cancel()

			// Create timeout-aware response writer.
			tw := &timeoutWriter{
				ResponseWriter: w,
			}

			// Channel to signal handler completion.
			done := make(chan struct{})

			// Run handler in goroutine.
			go func() {
				defer close(done)
				next.ServeHTTP(tw, r.WithContext(ctx))
			}()

			// Wait for completion or timeout.
			select {
			case <-done:
				// Handler completed normally.
				return

			case <-ctx.Done():
				// Timeout occurred.
				tw.setTimedOut()

				// Only write timeout response if handler hasn't written yet.
				if !tw.hasWritten() {
					appErr := middleware.RequestTimeout()
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.Header().Set("X-Content-Type-Options", "nosniff")

					// Use the middleware HTTPStatus for our custom error code.
					w.WriteHeader(middleware.HTTPStatus(appErr.Code()))

					// Write the error response.
					if writeErr := errors.FromError(appErr).WriteJSON(w); writeErr != nil {
						http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
					}
				}
				return
			}
		})
	}
}
