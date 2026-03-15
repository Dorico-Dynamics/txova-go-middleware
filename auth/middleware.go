package auth

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/Dorico-Dynamics/txova-go-core/errors"
	"github.com/Dorico-Dynamics/txova-go-core/logging"
	"github.com/Dorico-Dynamics/txova-go-security/audit"

	middleware "github.com/Dorico-Dynamics/txova-go-middleware"
)

// Config holds configuration for the authentication middleware.
type Config struct {
	// Validator is the JWT validator.
	Validator TokenValidator

	// Logger for logging authentication events (optional).
	Logger *logging.Logger

	// AuditLogger for security audit logging (optional).
	// Uses txova-go-security/audit for PII-masked security event logging.
	AuditLogger *audit.Logger

	// ExcludePaths are paths that bypass authentication.
	ExcludePaths []string

	// ExcludePatterns are regex patterns for paths that bypass authentication.
	ExcludePatterns []string
}

// Option is a functional option for configuring the middleware.
type Option func(*Config)

// WithLogger sets the logger for authentication events.
func WithLogger(logger *logging.Logger) Option {
	return func(c *Config) {
		c.Logger = logger
	}
}

// WithAuditLogger sets the security audit logger for authentication events.
// The audit logger automatically masks PII in log output.
func WithAuditLogger(auditLogger *audit.Logger) Option {
	return func(c *Config) {
		c.AuditLogger = auditLogger
	}
}

// WithExcludePaths sets paths that bypass authentication.
func WithExcludePaths(paths ...string) Option {
	return func(c *Config) {
		c.ExcludePaths = paths
	}
}

// WithExcludePatterns sets regex patterns for paths that bypass authentication.
func WithExcludePatterns(patterns ...string) Option {
	return func(c *Config) {
		c.ExcludePatterns = patterns
	}
}

// Middleware returns an HTTP middleware that enforces JWT authentication.
// Requests without a valid token receive a 401 Unauthorized response.
// If validator is nil, returns a middleware that responds with HTTP 500 for all requests.
func Middleware(validator TokenValidator, opts ...Option) func(http.Handler) http.Handler {
	cfg := Config{
		Validator: validator,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	// Guard against nil validator.
	if cfg.Validator == nil {
		if cfg.Logger != nil {
			cfg.Logger.Error("auth middleware initialized with nil validator")
		}
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeError(w, errors.InternalError("authentication service unavailable"))
			})
		}
	}

	// Build exclude path set.
	excludeSet := make(map[string]bool, len(cfg.ExcludePaths))
	for _, p := range cfg.ExcludePaths {
		excludeSet[p] = true
	}

	// Compile exclude patterns.
	var excludePatterns []*regexp.Regexp
	for _, pattern := range cfg.ExcludePatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			if cfg.Logger != nil {
				cfg.Logger.Error("failed to compile exclude pattern",
					"pattern", pattern,
					"error", err.Error(),
				)
			}
			continue
		}
		excludePatterns = append(excludePatterns, re)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path should be excluded.
			if shouldExclude(r.URL.Path, excludeSet, excludePatterns) {
				next.ServeHTTP(w, r)
				return
			}

			// Extract token from Authorization header.
			tokenString, err := extractBearerToken(r)
			if err != nil {
				logAuthFailure(cfg.Logger, cfg.AuditLogger, r, "missing or invalid authorization header")
				writeError(w, err)
				return
			}

			// Validate the token.
			claims, validateErr := cfg.Validator.ValidateToken(r.Context(), tokenString)
			if validateErr != nil {
				logAuthFailure(cfg.Logger, cfg.AuditLogger, r, validateErr.Error())
				writeError(w, validateErr)
				return
			}

			// Inject claims into context.
			ctx := WithClaims(r.Context(), claims)

			// Also inject user ID and roles for convenience.
			if claims.UserID != "" {
				ctx = middleware.WithUserID(ctx, claims.UserID)
			}
			if claims.UserType != "" {
				ctx = middleware.WithUserType(ctx, claims.UserType)
			}
			if len(claims.Roles) > 0 {
				ctx = middleware.WithRoles(ctx, claims.Roles)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// shouldExclude checks if a path should bypass authentication.
func shouldExclude(path string, excludeSet map[string]bool, patterns []*regexp.Regexp) bool {
	if excludeSet[path] {
		return true
	}

	for _, re := range patterns {
		if re.MatchString(path) {
			return true
		}
	}

	return false
}

// extractBearerToken extracts the JWT token from the Authorization header.
func extractBearerToken(r *http.Request) (string, *errors.AppError) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", middleware.TokenRequired()
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.TokenInvalid("authorization header must be 'Bearer {token}'")
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.TokenInvalid("token is empty")
	}

	return token, nil
}

// writeError writes an authentication error response.
func writeError(w http.ResponseWriter, appErr error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("WWW-Authenticate", "Bearer")

	err := errors.FromError(appErr)

	// Determine HTTP status - check middleware codes first, then fall back to core.
	status := middleware.HTTPStatus(err.Code())
	if status == http.StatusInternalServerError {
		// Not a middleware code, use the core error's status.
		status = err.HTTPStatus()
	}

	w.WriteHeader(status)
	if encodeErr := json.NewEncoder(w).Encode(err.ToResponse()); encodeErr != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	}
}

// logAuthFailure logs an authentication failure.
func logAuthFailure(logger *logging.Logger, auditLogger *audit.Logger, r *http.Request, reason string) {
	ip := middleware.GetClientIP(r)
	userAgent := r.UserAgent()

	// Log to standard logger if available.
	if logger != nil {
		requestID := middleware.RequestIDFromContext(r.Context())
		logger.Warn("authentication failed",
			"reason", reason,
			"path", r.URL.Path,
			"method", r.Method,
			"ip", ip,
			"request_id", requestID,
		)
	}

	// Log to audit logger if available (with automatic PII masking).
	if auditLogger != nil {
		auditLogger.LogLoginFailed(r.Context(), "", ip, userAgent, reason)
	}
}
