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

// recoveryWriter wraps http.ResponseWriter to track if headers/body have been written.
// This prevents the recovery handler from writing to a response that's already started.
type recoveryWriter struct {
	http.ResponseWriter
	written bool
	status  int
}

// WriteHeader implements http.ResponseWriter.
func (rw *recoveryWriter) WriteHeader(statusCode int) {
	if !rw.written {
		rw.status = statusCode
		rw.written = true
		rw.ResponseWriter.WriteHeader(statusCode)
	}
}

// Write implements http.ResponseWriter.
func (rw *recoveryWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
		rw.status = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

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
			// Wrap the response writer to track if headers/body have been written.
			rw := &recoveryWriter{ResponseWriter: w}

			defer func() {
				if rec := recover(); rec != nil {
					handlePanic(rec, r, w, rw, logger, &cfg)
				}
			}()

			next.ServeHTTP(rw, r)
		})
	}
}

// handlePanic handles a recovered panic by logging and optionally writing an error response.
func handlePanic(rec any, r *http.Request, w http.ResponseWriter, rw *recoveryWriter, logger *logging.Logger, cfg *Config) {
	// Capture stack trace.
	stack := make([]byte, cfg.StackSize)
	n := runtime.Stack(stack, false)
	stack = stack[:n]

	// Extract request context for logging.
	requestID := middleware.RequestIDFromContext(r.Context())
	userID, _ := middleware.UserIDFromContext(r.Context())

	// Log the panic.
	logPanic(logger, r, rec, stack, requestID, userID, rw, cfg.PrintStack)

	// Only write error response if headers haven't been sent yet.
	// If the response has already started, we can't change the status code
	// and writing would result in corrupted output.
	if rw.written {
		// Response already started - nothing we can safely do.
		// The panic is logged above for debugging.
		return
	}

	// Return 500 Internal Server Error without exposing details.
	writeErrorResponse(w)
}

// logPanic logs the panic with appropriate context.
func logPanic(logger *logging.Logger, r *http.Request, rec any, stack []byte, requestID, userID string, rw *recoveryWriter, printStack bool) {
	if logger == nil {
		return
	}

	logCtx := logger.WithContext(r.Context())
	args := []any{
		"panic", rec,
		"method", r.Method,
		"path", r.URL.Path,
		"request_id", requestID,
		"user_id", userID,
		"response_started", rw.written,
		"response_status", rw.status,
	}
	if printStack {
		args = append(args, "stack", string(stack))
	}
	logCtx.Error("panic recovered", args...)
}

// writeErrorResponse writes a 500 Internal Server Error response.
func writeErrorResponse(w http.ResponseWriter) {
	appErr := errors.InternalError("an internal error occurred")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Attempt to write the error response.
	if err := appErr.WriteJSON(w); err != nil {
		// If we can't write JSON, at least set the status.
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
