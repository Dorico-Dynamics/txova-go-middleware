package middleware

import (
	"context"
)

// ContextKey is a type for context keys to avoid collisions.
type ContextKey int

// Context keys for middleware values.
const (
	contextKeyUserID ContextKey = iota
	contextKeyUserType
	contextKeyRoles
	// ContextKeyClaims is the context key for storing JWT claims.
	// It is exported for use in the auth package.
	ContextKeyClaims
	contextKeyRequestID
)

// UserIDFromContext extracts the user ID from the context.
// Returns the user ID and true if found, empty string and false otherwise.
func UserIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	v, ok := ctx.Value(contextKeyUserID).(string)
	return v, ok
}

// WithUserID returns a new context with the user ID set.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, contextKeyUserID, userID)
}

// UserTypeFromContext extracts the user type from the context.
// Returns the user type and true if found, empty string and false otherwise.
func UserTypeFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	v, ok := ctx.Value(contextKeyUserType).(string)
	return v, ok
}

// WithUserType returns a new context with the user type set.
func WithUserType(ctx context.Context, userType string) context.Context {
	return context.WithValue(ctx, contextKeyUserType, userType)
}

// RolesFromContext extracts the roles from the context.
// Returns the roles slice and true if found, nil and false otherwise.
func RolesFromContext(ctx context.Context) ([]string, bool) {
	if ctx == nil {
		return nil, false
	}
	v, ok := ctx.Value(contextKeyRoles).([]string)
	return v, ok
}

// WithRoles returns a new context with the roles set.
func WithRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, contextKeyRoles, roles)
}

// RequestIDFromContext extracts the request ID from the context.
// Returns the request ID if found, empty string otherwise.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, ok := ctx.Value(contextKeyRequestID).(string)
	if !ok {
		return ""
	}
	return v
}

// WithRequestID returns a new context with the request ID set.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, contextKeyRequestID, requestID)
}
