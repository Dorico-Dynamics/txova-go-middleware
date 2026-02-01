package auth

import (
	"net/http"
	"strings"

	middleware "github.com/Dorico-Dynamics/txova-go-middleware"
)

// OptionalMiddleware returns an HTTP middleware that attempts JWT authentication
// but continues even if no token is present or the token is invalid.
// If a valid token is present, claims are injected into the context.
// If no token or invalid token, the request continues without claims.
func OptionalMiddleware(validator *Validator, opts ...Option) func(http.Handler) http.Handler {
	cfg := Config{
		Validator: validator,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try to extract token from Authorization header.
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// No token, continue without claims.
				next.ServeHTTP(w, r)
				return
			}

			// Try to parse Bearer token.
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				// Invalid format, continue without claims.
				next.ServeHTTP(w, r)
				return
			}

			tokenString := strings.TrimSpace(parts[1])
			if tokenString == "" {
				// Empty token, continue without claims.
				next.ServeHTTP(w, r)
				return
			}

			// Attempt to validate the token.
			claims, err := cfg.Validator.Validate(tokenString)
			if err != nil {
				// Invalid token, continue without claims.
				next.ServeHTTP(w, r)
				return
			}

			// Valid token - inject claims into context.
			ctx := WithClaims(r.Context(), claims)

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
