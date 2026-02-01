// Package auth provides JWT authentication middleware for the Txova platform.
package auth

import (
	"context"

	"github.com/golang-jwt/jwt/v5"

	middleware "github.com/Dorico-Dynamics/txova-go-middleware"
)

// Claims represents the JWT claims for the Txova platform.
type Claims struct {
	jwt.RegisteredClaims

	// UserID is the authenticated user's ID.
	UserID string `json:"user_id,omitempty"`

	// UserType is the type of user (rider, driver, admin).
	UserType string `json:"user_type,omitempty"`

	// Roles are the permission roles assigned to the user.
	Roles []string `json:"roles,omitempty"`

	// Permissions are the specific permissions assigned to the user.
	Permissions []string `json:"permissions,omitempty"`
}

// HasRole checks if the claims include the specified role.
func (c *Claims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks if the claims include any of the specified roles.
func (c *Claims) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if c.HasRole(role) {
			return true
		}
	}
	return false
}

// HasPermission checks if the claims include the specified permission.
func (c *Claims) HasPermission(permission string) bool {
	for _, p := range c.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// HasAllPermissions checks if the claims include all of the specified permissions.
func (c *Claims) HasAllPermissions(permissions ...string) bool {
	for _, perm := range permissions {
		if !c.HasPermission(perm) {
			return false
		}
	}
	return true
}

// ClaimsFromContext extracts the Claims from the request context.
// Returns the claims and true if found, nil and false otherwise.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	if ctx == nil {
		return nil, false
	}
	claims, ok := ctx.Value(middleware.ContextKeyClaims).(*Claims)
	return claims, ok
}

// WithClaims returns a new context with the claims set.
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, middleware.ContextKeyClaims, claims)
}
