// Package recovery provides middleware for recovering from panics in HTTP handlers.
package recovery

import (
	"net/http"
	"runtime"

	"github.com/Dorico-Dynamics/txova-go-core/errors"
	"github.com/Dorico-Dynamics/txova-go-core/logging"

	middleware "github.com/Dorico-Dynamics/txova-go-middleware"
)

// DefaultStackSize is the default size of the stack trace buffer.
const DefaultStackSize = 4096

// Config holds configuration for the recovery middleware.
type Config struct {
	// StackSize is the size of the buffer for capturing stack traces.
	// Defaults to 4096 bytes.
	StackSize int

	// PrintStack determines whether to include stack traces in logs.
	// Defaults to true.
	PrintStack bool
}

// Option is a functional option for configuring the middleware.
type Option func(*Config)

// WithStackSize sets the stack trace buffer size.
func WithStackSize(size int) Option {
	return func(c *Config) {
		c.StackSize = size
	}
}

// WithPrintStack enables or disables stack trace logging.
func WithPrintStack(enabled bool) Option {
	return func(c *Config) {
		c.PrintStack = enabled
	}
}

// Middleware returns an HTTP middleware that recovers from panics.
// On panic, it logs the error with full stack trace and returns a 500 Internal Server Error.
// Panic details are never exposed to the client.
func Middleware(logger *logging.Logger, opts ...Option) func(http.Handler) http.Handler {
	cfg := Config{
		StackSize:  DefaultStackSize,
		PrintStack: true,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					// Capture stack trace.
					stack := make([]byte, cfg.StackSize)
					n := runtime.Stack(stack, false)
					stack = stack[:n]

					// Extract request context for logging.
					requestID := middleware.RequestIDFromContext(r.Context())
					userID, _ := middleware.UserIDFromContext(r.Context())

					// Log the panic.
					logCtx := logger.WithContext(r.Context())
					if cfg.PrintStack {
						logCtx.Error("panic recovered",
							"panic", rec,
							"stack", string(stack),
							"method", r.Method,
							"path", r.URL.Path,
							"request_id", requestID,
							"user_id", userID,
						)
					} else {
						logCtx.Error("panic recovered",
							"panic", rec,
							"method", r.Method,
							"path", r.URL.Path,
							"request_id", requestID,
							"user_id", userID,
						)
					}

					// Return 500 Internal Server Error without exposing details.
					appErr := errors.InternalError("an internal error occurred")
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.Header().Set("X-Content-Type-Options", "nosniff")

					// Attempt to write the error response.
					if err := appErr.WriteJSON(w); err != nil {
						// If we can't write JSON, at least set the status.
						http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					}
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
