package middleware

import (
	"context"
	"testing"
)

func TestUserIDFromContext(t *testing.T) {
	tests := []struct {
		name       string
		ctx        context.Context
		wantUserID string
		wantOK     bool
	}{
		{
			name:       "user ID present",
			ctx:        WithUserID(context.Background(), "user-123"),
			wantUserID: "user-123",
			wantOK:     true,
		},
		{
			name:       "user ID not present",
			ctx:        context.Background(),
			wantUserID: "",
			wantOK:     false,
		},
		{
			name:       "nil context",
			ctx:        nil,
			wantUserID: "",
			wantOK:     false,
		},
		{
			name:       "empty user ID",
			ctx:        WithUserID(context.Background(), ""),
			wantUserID: "",
			wantOK:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUserID, gotOK := UserIDFromContext(tt.ctx)
			if gotUserID != tt.wantUserID {
				t.Errorf("UserIDFromContext() userID = %q, want %q", gotUserID, tt.wantUserID)
			}
			if gotOK != tt.wantOK {
				t.Errorf("UserIDFromContext() ok = %v, want %v", gotOK, tt.wantOK)
			}
		})
	}
}

func TestUserTypeFromContext(t *testing.T) {
	tests := []struct {
		name         string
		ctx          context.Context
		wantUserType string
		wantOK       bool
	}{
		{
			name:         "user type present",
			ctx:          WithUserType(context.Background(), "admin"),
			wantUserType: "admin",
			wantOK:       true,
		},
		{
			name:         "user type not present",
			ctx:          context.Background(),
			wantUserType: "",
			wantOK:       false,
		},
		{
			name:         "nil context",
			ctx:          nil,
			wantUserType: "",
			wantOK:       false,
		},
		{
			name:         "rider user type",
			ctx:          WithUserType(context.Background(), "rider"),
			wantUserType: "rider",
			wantOK:       true,
		},
		{
			name:         "driver user type",
			ctx:          WithUserType(context.Background(), "driver"),
			wantUserType: "driver",
			wantOK:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUserType, gotOK := UserTypeFromContext(tt.ctx)
			if gotUserType != tt.wantUserType {
				t.Errorf("UserTypeFromContext() userType = %q, want %q", gotUserType, tt.wantUserType)
			}
			if gotOK != tt.wantOK {
				t.Errorf("UserTypeFromContext() ok = %v, want %v", gotOK, tt.wantOK)
			}
		})
	}
}

func TestRolesFromContext(t *testing.T) {
	tests := []struct {
		name      string
		ctx       context.Context
		wantRoles []string
		wantOK    bool
	}{
		{
			name:      "roles present",
			ctx:       WithRoles(context.Background(), []string{"admin", "user"}),
			wantRoles: []string{"admin", "user"},
			wantOK:    true,
		},
		{
			name:      "roles not present",
			ctx:       context.Background(),
			wantRoles: nil,
			wantOK:    false,
		},
		{
			name:      "nil context",
			ctx:       nil,
			wantRoles: nil,
			wantOK:    false,
		},
		{
			name:      "empty roles slice",
			ctx:       WithRoles(context.Background(), []string{}),
			wantRoles: []string{},
			wantOK:    true,
		},
		{
			name:      "single role",
			ctx:       WithRoles(context.Background(), []string{"viewer"}),
			wantRoles: []string{"viewer"},
			wantOK:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRoles, gotOK := RolesFromContext(tt.ctx)

			if tt.wantRoles == nil {
				if gotRoles != nil {
					t.Errorf("RolesFromContext() roles = %v, want nil", gotRoles)
				}
			} else {
				if len(gotRoles) != len(tt.wantRoles) {
					t.Errorf("RolesFromContext() roles length = %d, want %d", len(gotRoles), len(tt.wantRoles))
				} else {
					for i, role := range gotRoles {
						if role != tt.wantRoles[i] {
							t.Errorf("RolesFromContext() roles[%d] = %q, want %q", i, role, tt.wantRoles[i])
						}
					}
				}
			}

			if gotOK != tt.wantOK {
				t.Errorf("RolesFromContext() ok = %v, want %v", gotOK, tt.wantOK)
			}
		})
	}
}

func TestRequestIDFromContext(t *testing.T) {
	tests := []struct {
		name          string
		ctx           context.Context
		wantRequestID string
	}{
		{
			name:          "request ID present",
			ctx:           WithRequestID(context.Background(), "req-abc-123"),
			wantRequestID: "req-abc-123",
		},
		{
			name:          "request ID not present",
			ctx:           context.Background(),
			wantRequestID: "",
		},
		{
			name:          "nil context",
			ctx:           nil,
			wantRequestID: "",
		},
		{
			name:          "UUID format request ID",
			ctx:           WithRequestID(context.Background(), "550e8400-e29b-41d4-a716-446655440000"),
			wantRequestID: "550e8400-e29b-41d4-a716-446655440000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRequestID := RequestIDFromContext(tt.ctx)
			if gotRequestID != tt.wantRequestID {
				t.Errorf("RequestIDFromContext() = %q, want %q", gotRequestID, tt.wantRequestID)
			}
		})
	}
}

func TestContextChaining(t *testing.T) {
	// Test that multiple context values can be chained together.
	ctx := context.Background()
	ctx = WithUserID(ctx, "user-456")
	ctx = WithUserType(ctx, "driver")
	ctx = WithRoles(ctx, []string{"driver", "premium"})
	ctx = WithRequestID(ctx, "req-789")

	userID, ok := UserIDFromContext(ctx)
	if !ok || userID != "user-456" {
		t.Errorf("UserIDFromContext() after chaining = (%q, %v), want (%q, true)", userID, ok, "user-456")
	}

	userType, ok := UserTypeFromContext(ctx)
	if !ok || userType != "driver" {
		t.Errorf("UserTypeFromContext() after chaining = (%q, %v), want (%q, true)", userType, ok, "driver")
	}

	roles, ok := RolesFromContext(ctx)
	if !ok || len(roles) != 2 {
		t.Errorf("RolesFromContext() after chaining = (%v, %v), want ([driver premium], true)", roles, ok)
	}

	requestID := RequestIDFromContext(ctx)
	if requestID != "req-789" {
		t.Errorf("RequestIDFromContext() after chaining = %q, want %q", requestID, "req-789")
	}
}

func TestContextKeyClaims(t *testing.T) {
	// Verify the claims context key is exported and usable.
	ctx := context.Background()
	claims := map[string]string{"user_id": "123"}
	ctx = context.WithValue(ctx, ContextKeyClaims, claims)

	retrieved, ok := ctx.Value(ContextKeyClaims).(map[string]string)
	if !ok {
		t.Fatal("ContextKeyClaims value not retrievable")
	}
	if retrieved["user_id"] != "123" {
		t.Errorf("ContextKeyClaims value = %v, want user_id=123", retrieved)
	}
}
