package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	middleware "github.com/Dorico-Dynamics/txova-go-middleware"
)

func TestOptionalMiddleware_NoToken(t *testing.T) {
	validator, _ := setupTestValidator(t)

	var claimsFound bool
	handler := OptionalMiddleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, claimsFound = ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if claimsFound {
		t.Error("claims should not be present without token")
	}
}

func TestOptionalMiddleware_InvalidFormat(t *testing.T) {
	validator, _ := setupTestValidator(t)

	tests := []struct {
		name   string
		header string
	}{
		{"Basic auth", "Basic dXNlcjpwYXNz"},
		{"No Bearer prefix", "some-token"},
		{"Bearer only", "Bearer"},
		{"Bearer with spaces only", "Bearer   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var claimsFound bool
			handler := OptionalMiddleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, claimsFound = ClaimsFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
			req.Header.Set("Authorization", tt.header)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}

			if claimsFound {
				t.Error("claims should not be present with invalid format")
			}
		})
	}
}

func TestOptionalMiddleware_InvalidToken(t *testing.T) {
	validator, _ := setupTestValidator(t)

	var claimsFound bool
	handler := OptionalMiddleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, claimsFound = ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if claimsFound {
		t.Error("claims should not be present with invalid token")
	}
}

func TestOptionalMiddleware_ExpiredToken(t *testing.T) {
	validator, rsaKey := setupTestValidator(t)

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
		UserID: "expired-user",
	}
	tokenString := createTestToken(t, claims, rsaKey, jwt.SigningMethodRS256)

	var claimsFound bool
	handler := OptionalMiddleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, claimsFound = ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if claimsFound {
		t.Error("claims should not be present with expired token")
	}
}

func TestOptionalMiddleware_ValidToken(t *testing.T) {
	validator, rsaKey := setupTestValidator(t)

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID:   "user-123",
		UserType: "rider",
		Roles:    []string{"user", "verified"},
	}
	tokenString := createTestToken(t, claims, rsaKey, jwt.SigningMethodRS256)

	var capturedClaims *Claims
	var capturedUserID string
	var capturedUserType string
	var capturedRoles []string

	handler := OptionalMiddleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedClaims, _ = ClaimsFromContext(r.Context())
		capturedUserID, _ = middleware.UserIDFromContext(r.Context())
		capturedUserType, _ = middleware.UserTypeFromContext(r.Context())
		capturedRoles, _ = middleware.RolesFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if capturedClaims == nil {
		t.Fatal("claims should be present with valid token")
	}

	if capturedClaims.UserID != claims.UserID {
		t.Errorf("UserID = %q, want %q", capturedClaims.UserID, claims.UserID)
	}

	if capturedUserID != claims.UserID {
		t.Errorf("context UserID = %q, want %q", capturedUserID, claims.UserID)
	}

	if capturedUserType != claims.UserType {
		t.Errorf("context UserType = %q, want %q", capturedUserType, claims.UserType)
	}

	if len(capturedRoles) != len(claims.Roles) {
		t.Errorf("context Roles length = %d, want %d", len(capturedRoles), len(claims.Roles))
	}
}

func TestOptionalMiddleware_ValidTokenEmptyOptionalFields(t *testing.T) {
	validator, rsaKey := setupTestValidator(t)

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID: "user-456",
		// UserType and Roles are empty.
	}
	tokenString := createTestToken(t, claims, rsaKey, jwt.SigningMethodRS256)

	var capturedUserType string
	var capturedRoles []string
	var userTypeFound, rolesFound bool

	handler := OptionalMiddleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserType, userTypeFound = middleware.UserTypeFromContext(r.Context())
		capturedRoles, rolesFound = middleware.RolesFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if userTypeFound {
		t.Errorf("UserType should not be in context when empty, got %q", capturedUserType)
	}

	if rolesFound {
		t.Errorf("Roles should not be in context when empty, got %v", capturedRoles)
	}
}

func TestOptionalMiddleware_BearerCaseInsensitive(t *testing.T) {
	validator, rsaKey := setupTestValidator(t)

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID: "user-123",
	}
	tokenString := createTestToken(t, claims, rsaKey, jwt.SigningMethodRS256)

	tests := []struct {
		name   string
		prefix string
	}{
		{"lowercase", "bearer"},
		{"uppercase", "BEARER"},
		{"mixed case", "BeArEr"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var claimsFound bool
			handler := OptionalMiddleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, claimsFound = ClaimsFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
			req.Header.Set("Authorization", tt.prefix+" "+tokenString)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}

			if !claimsFound {
				t.Error("claims should be present with valid token")
			}
		})
	}
}
