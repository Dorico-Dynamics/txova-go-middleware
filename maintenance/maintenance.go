// Package maintenance provides maintenance mode middleware for the Txova platform.
package maintenance

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Dorico-Dynamics/txova-go-core/errors"
)

// FlagStore defines the interface for checking maintenance mode status.
// This interface is typically implemented using Redis or another distributed store.
type FlagStore interface {
	// IsEnabled returns true if maintenance mode is currently enabled.
	IsEnabled(ctx context.Context) (bool, error)
	// GetMessage returns the maintenance message to display to users.
	GetMessage(ctx context.Context) (string, error)
	// GetEndTime returns the expected end time of maintenance, if known.
	GetEndTime(ctx context.Context) (*time.Time, error)
}

// Config holds configuration for the maintenance mode middleware.
type Config struct {
	// BypassIPs are IP addresses that bypass maintenance mode.
	BypassIPs []string

	// BypassPaths are paths that bypass maintenance mode.
	// /health is always included by default.
	BypassPaths []string

	// DefaultMessage is the message to use when none is set in the store.
	DefaultMessage string
}

// Option is a functional option for configuring the maintenance middleware.
type Option func(*Config)

// WithBypassIPs sets the IP addresses that bypass maintenance mode.
func WithBypassIPs(ips ...string) Option {
	return func(c *Config) {
		c.BypassIPs = ips
	}
}

// WithBypassPaths sets the paths that bypass maintenance mode.
func WithBypassPaths(paths ...string) Option {
	return func(c *Config) {
		c.BypassPaths = paths
	}
}

// WithDefaultMessage sets the default maintenance message.
func WithDefaultMessage(message string) Option {
	return func(c *Config) {
		c.DefaultMessage = message
	}
}

// Response is the response body during maintenance mode.
type Response struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	EndTime *time.Time `json:"end_time,omitempty"`
}

// Middleware returns an HTTP middleware that blocks requests during maintenance mode.
// Requests from bypass IPs or to bypass paths are allowed through.
func Middleware(store FlagStore, opts ...Option) func(http.Handler) http.Handler {
	cfg := Config{
		DefaultMessage: "Service is temporarily unavailable for maintenance",
		BypassPaths:    []string{"/health", "/ready"},
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	// Build bypass sets for fast lookup.
	bypassIPs := make(map[string]bool, len(cfg.BypassIPs))
	for _, ip := range cfg.BypassIPs {
		bypassIPs[ip] = true
	}

	bypassPaths := make(map[string]bool, len(cfg.BypassPaths))
	for _, path := range cfg.BypassPaths {
		bypassPaths[path] = true
	}
	// Always include health and ready endpoints.
	bypassPaths["/health"] = true
	bypassPaths["/ready"] = true

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check bypass paths first (fast path).
			if bypassPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Check bypass IPs.
			clientIP := getClientIP(r)
			if bypassIPs[clientIP] {
				next.ServeHTTP(w, r)
				return
			}

			// Check maintenance status.
			enabled, err := store.IsEnabled(r.Context())
			if err != nil {
				// On error checking status, allow the request through.
				// This prevents maintenance check failures from blocking traffic.
				next.ServeHTTP(w, r)
				return
			}

			if !enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Maintenance mode is enabled - block the request.
			message := cfg.DefaultMessage
			if storeMessage, err := store.GetMessage(r.Context()); err == nil && storeMessage != "" {
				message = storeMessage
			}

			var endTime *time.Time
			if t, err := store.GetEndTime(r.Context()); err == nil {
				endTime = t
			}

			writeMaintenanceResponse(w, message, endTime)
		})
	}
}

// writeMaintenanceResponse writes a maintenance mode response.
func writeMaintenanceResponse(w http.ResponseWriter, message string, endTime *time.Time) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Retry-After", "300") // Suggest retry after 5 minutes.

	appErr := errors.ServiceUnavailable(message)
	w.WriteHeader(appErr.HTTPStatus())

	resp := Response{
		EndTime: endTime,
	}
	resp.Error.Code = "SERVICE_UNAVAILABLE"
	resp.Error.Message = message

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
	}
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
