package auth

import (
	"context"
	"testing"
)

func TestClaims_HasRole(t *testing.T) {
	claims := &Claims{
		Roles: []string{"admin", "user", "moderator"},
	}

	tests := []struct {
		role string
		want bool
	}{
		{"admin", true},
		{"user", true},
		{"moderator", true},
		{"superadmin", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			got := claims.HasRole(tt.role)
			if got != tt.want {
				t.Errorf("HasRole(%q) = %v, want %v", tt.role, got, tt.want)
			}
		})
	}
}

func TestClaims_HasRole_Empty(t *testing.T) {
	claims := &Claims{Roles: nil}

	if claims.HasRole("admin") {
		t.Error("HasRole should return false for nil roles")
	}

	claims.Roles = []string{}
	if claims.HasRole("admin") {
		t.Error("HasRole should return false for empty roles")
	}
}

func TestClaims_HasAnyRole(t *testing.T) {
	claims := &Claims{
		Roles: []string{"user", "viewer"},
	}

	tests := []struct {
		name  string
		roles []string
		want  bool
	}{
		{"has first role", []string{"user", "admin"}, true},
		{"has second role", []string{"admin", "viewer"}, true},
		{"has no roles", []string{"admin", "superuser"}, false},
		{"empty input", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := claims.HasAnyRole(tt.roles...)
			if got != tt.want {
				t.Errorf("HasAnyRole(%v) = %v, want %v", tt.roles, got, tt.want)
			}
		})
	}
}

func TestClaims_HasPermission(t *testing.T) {
	claims := &Claims{
		Permissions: []string{"read:users", "write:users", "delete:users"},
	}

	tests := []struct {
		permission string
		want       bool
	}{
		{"read:users", true},
		{"write:users", true},
		{"delete:users", true},
		{"read:admin", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.permission, func(t *testing.T) {
			got := claims.HasPermission(tt.permission)
			if got != tt.want {
				t.Errorf("HasPermission(%q) = %v, want %v", tt.permission, got, tt.want)
			}
		})
	}
}

func TestClaims_HasAllPermissions(t *testing.T) {
	claims := &Claims{
		Permissions: []string{"read:users", "write:users", "delete:users"},
	}

	tests := []struct {
		name        string
		permissions []string
		want        bool
	}{
		{"has all", []string{"read:users", "write:users"}, true},
		{"has single", []string{"read:users"}, true},
		{"missing one", []string{"read:users", "read:admin"}, false},
		{"missing all", []string{"read:admin", "write:admin"}, false},
		{"empty input", []string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := claims.HasAllPermissions(tt.permissions...)
			if got != tt.want {
				t.Errorf("HasAllPermissions(%v) = %v, want %v", tt.permissions, got, tt.want)
			}
		})
	}
}

func TestClaimsFromContext(t *testing.T) {
	claims := &Claims{
		UserID:   "user-123",
		UserType: "admin",
		Roles:    []string{"admin"},
	}

	tests := []struct {
		name    string
		ctx     context.Context
		wantNil bool
		wantOK  bool
	}{
		{
			name:    "claims present",
			ctx:     WithClaims(context.Background(), claims),
			wantNil: false,
			wantOK:  true,
		},
		{
			name:    "claims not present",
			ctx:     context.Background(),
			wantNil: true,
			wantOK:  false,
		},
		{
			name:    "nil context",
			ctx:     nil,
			wantNil: true,
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ClaimsFromContext(tt.ctx)

			if ok != tt.wantOK {
				t.Errorf("ClaimsFromContext() ok = %v, want %v", ok, tt.wantOK)
			}

			if tt.wantNil && got != nil {
				t.Errorf("ClaimsFromContext() = %v, want nil", got)
			}

			if !tt.wantNil && got == nil {
				t.Error("ClaimsFromContext() = nil, want non-nil")
			}

			if got != nil && got.UserID != claims.UserID {
				t.Errorf("ClaimsFromContext().UserID = %q, want %q", got.UserID, claims.UserID)
			}
		})
	}
}

func TestWithClaims(t *testing.T) {
	claims := &Claims{
		UserID:   "user-456",
		UserType: "driver",
	}

	ctx := WithClaims(context.Background(), claims)

	got, ok := ClaimsFromContext(ctx)
	if !ok {
		t.Fatal("ClaimsFromContext() returned false")
	}

	if got.UserID != claims.UserID {
		t.Errorf("UserID = %q, want %q", got.UserID, claims.UserID)
	}

	if got.UserType != claims.UserType {
		t.Errorf("UserType = %q, want %q", got.UserType, claims.UserType)
	}
}
