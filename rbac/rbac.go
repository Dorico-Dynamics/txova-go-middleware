// Package rbac provides role-based access control (RBAC) middleware for the Txova platform.
package rbac

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Dorico-Dynamics/txova-go-core/errors"
	"github.com/Dorico-Dynamics/txova-go-core/logging"
	"github.com/Dorico-Dynamics/txova-go-security/audit"

	middleware "github.com/Dorico-Dynamics/txova-go-middleware"
	"github.com/Dorico-Dynamics/txova-go-middleware/auth"
)

// Config holds configuration for RBAC middleware.
type Config struct {
	// Logger for logging access denial events (optional).
	Logger *logging.Logger

	// AuditLogger for security audit logging (optional).
	// Uses txova-go-security/audit for PII-masked security event logging.
	AuditLogger *audit.Logger
}

// Option is a functional option for configuring RBAC middleware.
type Option func(*Config)

// WithLogger sets the logger for access denial events.
func WithLogger(logger *logging.Logger) Option {
	return func(c *Config) {
		c.Logger = logger
	}
}

// WithAuditLogger sets the security audit logger for access denial events.
// The audit logger automatically masks PII in log output.
func WithAuditLogger(auditLogger *audit.Logger) Option {
	return func(c *Config) {
		c.AuditLogger = auditLogger
	}
}

// RequireRole returns a middleware that requires the authenticated user
// to have at least one of the specified roles.
// Returns 403 Forbidden if the user doesn't have any of the required roles.
// Returns 401 Unauthorized if no claims are present in context.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return RequireRoleWithOptions(roles)
}

// RequireRoleWithOptions returns a middleware that requires the authenticated user
// to have at least one of the specified roles, with additional options.
func RequireRoleWithOptions(roles []string, opts ...Option) func(http.Handler) http.Handler {
	cfg := Config{}
	for _, opt := range opts {
		opt(&cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok {
				writeError(w, errors.InvalidCredentials("authentication required"))
				return
			}

			if !claims.HasAnyRole(roles...) {
				logAccessDenial(&cfg, r, "missing required role")
				writeError(w, errors.Forbidden("insufficient permissions"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission returns a middleware that requires the authenticated user
// to have ALL of the specified permissions.
// Returns 403 Forbidden if the user is missing any required permission.
// Returns 401 Unauthorized if no claims are present in context.
func RequirePermission(permissions ...string) func(http.Handler) http.Handler {
	return RequirePermissionWithOptions(permissions)
}

// RequirePermissionWithOptions returns a middleware that requires the authenticated user
// to have ALL of the specified permissions, with additional options.
func RequirePermissionWithOptions(permissions []string, opts ...Option) func(http.Handler) http.Handler {
	cfg := Config{}
	for _, opt := range opts {
		opt(&cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok {
				writeError(w, errors.InvalidCredentials("authentication required"))
				return
			}

			if !claims.HasAllPermissions(permissions...) {
				logAccessDenial(&cfg, r, "missing required permission")
				writeError(w, errors.Forbidden("insufficient permissions"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireUserType returns a middleware that requires the authenticated user
// to have one of the specified user types.
// Returns 403 Forbidden if the user type doesn't match.
// Returns 401 Unauthorized if no claims are present in context.
func RequireUserType(types ...string) func(http.Handler) http.Handler {
	return RequireUserTypeWithOptions(types)
}

// RequireUserTypeWithOptions returns a middleware that requires the authenticated user
// to have one of the specified user types, with additional options.
func RequireUserTypeWithOptions(types []string, opts ...Option) func(http.Handler) http.Handler {
	cfg := Config{}
	for _, opt := range opts {
		opt(&cfg)
	}

	typeSet := make(map[string]bool, len(types))
	for _, t := range types {
		typeSet[t] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok {
				writeError(w, errors.InvalidCredentials("authentication required"))
				return
			}

			if !typeSet[claims.UserType] {
				logAccessDenial(&cfg, r, "invalid user type")
				writeError(w, errors.Forbidden("access denied for user type"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireOwner returns a middleware that requires the authenticated user
// to be the owner of the resource identified by the URL parameter.
// The paramName specifies which chi URL parameter contains the owner ID.
// Returns 403 Forbidden if the user is not the owner.
// Returns 401 Unauthorized if no claims are present in context.
func RequireOwner(paramName string) func(http.Handler) http.Handler {
	return RequireOwnerWithOptions(paramName)
}

// RequireOwnerWithOptions returns a middleware that requires the authenticated user
// to be the owner of the resource, with additional options.
func RequireOwnerWithOptions(paramName string, opts ...Option) func(http.Handler) http.Handler {
	cfg := Config{}
	for _, opt := range opts {
		opt(&cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok {
				writeError(w, errors.InvalidCredentials("authentication required"))
				return
			}

			ownerID := chi.URLParam(r, paramName)
			if ownerID == "" {
				writeError(w, errors.ValidationError("resource owner ID not found in URL"))
				return
			}

			if claims.UserID != ownerID {
				logAccessDenial(&cfg, r, "not resource owner")
				writeError(w, errors.Forbidden("access denied: not resource owner"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireRoleOrOwner returns a middleware that requires the authenticated user
// to either have one of the specified roles OR be the owner of the resource.
// This is useful for admin-or-owner access patterns.
// Returns 403 Forbidden if the user has neither role nor ownership.
// Returns 401 Unauthorized if no claims are present in context.
func RequireRoleOrOwner(paramName string, roles ...string) func(http.Handler) http.Handler {
	return RequireRoleOrOwnerWithOptions(paramName, roles)
}

// RequireRoleOrOwnerWithOptions returns a middleware that requires the authenticated user
// to either have one of the specified roles OR be the owner, with additional options.
func RequireRoleOrOwnerWithOptions(paramName string, roles []string, opts ...Option) func(http.Handler) http.Handler {
	cfg := Config{}
	for _, opt := range opts {
		opt(&cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok {
				writeError(w, errors.InvalidCredentials("authentication required"))
				return
			}

			// Check if user has required role.
			if claims.HasAnyRole(roles...) {
				next.ServeHTTP(w, r)
				return
			}

			// Check if user is owner.
			ownerID := chi.URLParam(r, paramName)
			if ownerID != "" && claims.UserID == ownerID {
				next.ServeHTTP(w, r)
				return
			}

			logAccessDenial(&cfg, r, "neither role nor owner")
			writeError(w, errors.Forbidden("access denied"))
		})
	}
}

// writeError writes an RBAC error response.
func writeError(w http.ResponseWriter, appErr *errors.AppError) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	w.WriteHeader(appErr.HTTPStatus())

	// Attempt to write JSON response. If encoding fails, write plain text fallback.
	// Do not call http.Error as it would invoke WriteHeader again.
	if err := json.NewEncoder(w).Encode(appErr.ToResponse()); err != nil {
		// WriteHeader already called, just write body. Ignore write error as
		// there's nothing we can do if writing the fallback also fails.
		//nolint:errcheck // Intentional - fallback write, nothing to do on error
		_, _ = w.Write([]byte(http.StatusText(appErr.HTTPStatus())))
	}
}

// logAccessDenial logs an access denial event.
func logAccessDenial(cfg *Config, r *http.Request, reason string) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	requestID := middleware.RequestIDFromContext(r.Context())
	ip := getClientIP(r)

	// Log to standard logger if available.
	if cfg.Logger != nil {
		cfg.Logger.Warn("access denied",
			"reason", reason,
			"path", r.URL.Path,
			"method", r.Method,
			"user_id", userID,
			"request_id", requestID,
		)
	}

	// Log to audit logger if available (with automatic PII masking).
	if cfg.AuditLogger != nil {
		cfg.AuditLogger.LogPermissionDenied(r.Context(), userID, r.URL.Path, r.Method, ip)
	}
}

// getClientIP extracts the client IP address from the request.
// Handles both IPv4 and IPv6 addresses correctly.
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

	// Use net.SplitHostPort to properly handle IPv4 and IPv6 addresses.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If SplitHostPort fails (e.g., no port), return the address as-is.
		// Trim brackets from bare IPv6 addresses like "[::1]".
		addr := r.RemoteAddr
		if strings.HasPrefix(addr, "[") && strings.HasSuffix(addr, "]") {
			return addr[1 : len(addr)-1]
		}
		return addr
	}

	return host
}
