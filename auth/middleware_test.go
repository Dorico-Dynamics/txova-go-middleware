package auth

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Dorico-Dynamics/txova-go-core/errors"

	middleware "github.com/Dorico-Dynamics/txova-go-middleware"
)

func setupTestValidator(t *testing.T) (*Validator, *rsa.PrivateKey) {
	t.Helper()
	rsaKey := generateTestRSAKey(t)
	validator, err := NewValidator(ValidatorConfig{
		PublicKey: &rsaKey.PublicKey,
	})
	if err != nil {
		t.Fatalf("NewValidator() error = %v", err)
	}
	return validator, rsaKey
}

func createValidTestToken(t *testing.T, key *rsa.PrivateKey, claims *Claims) string {
	t.Helper()
	if claims.ExpiresAt == nil {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Hour))
	}
	if claims.IssuedAt == nil {
		claims.IssuedAt = jwt.NewNumericDate(time.Now())
	}
	return createTestToken(t, claims, key, jwt.SigningMethodRS256)
}

func TestMiddleware_ValidToken(t *testing.T) {
	validator, rsaKey := setupTestValidator(t)

	claims := &Claims{
		UserID:      "user-123",
		UserType:    "rider",
		Roles:       []string{"user", "verified"},
		Permissions: []string{"trips:read"},
	}
	tokenString := createValidTestToken(t, rsaKey, claims)

	var capturedClaims *Claims
	var capturedUserID string
	var capturedUserType string
	var capturedRoles []string

	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		t.Fatal("claims not set in context")
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

func TestMiddleware_MissingAuthorizationHeader(t *testing.T) {
	validator, _ := setupTestValidator(t)

	handlerCalled := false
	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if handlerCalled {
		t.Error("handler should not be called without auth")
	}

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	if rec.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Errorf("WWW-Authenticate header = %q, want %q", rec.Header().Get("WWW-Authenticate"), "Bearer")
	}

	var errResp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}

	if errResp.Error.Code != string(middleware.CodeTokenRequired) {
		t.Errorf("error code = %v, want %v", errResp.Error.Code, middleware.CodeTokenRequired)
	}
}

func TestMiddleware_InvalidAuthorizationFormat(t *testing.T) {
	validator, _ := setupTestValidator(t)

	tests := []struct {
		name   string
		header string
	}{
		{"Basic auth", "Basic dXNlcjpwYXNz"},
		{"No Bearer prefix", "some-token"},
		{"Bearer only", "Bearer"},
		{"Bearer with spaces only", "Bearer   "},
		{"Wrong case", "bearer some-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
			req.Header.Set("Authorization", tt.header)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestMiddleware_ExpiredToken(t *testing.T) {
	validator, rsaKey := setupTestValidator(t)

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
		UserID: "expired-user",
	}
	tokenString := createTestToken(t, claims, rsaKey, jwt.SigningMethodRS256)

	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var errResp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}

	if errResp.Error.Code != string(errors.CodeTokenExpired) {
		t.Errorf("error code = %v, want %v", errResp.Error.Code, errors.CodeTokenExpired)
	}
}

func TestMiddleware_MalformedToken(t *testing.T) {
	validator, _ := setupTestValidator(t)

	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer not.a.valid.jwt.token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_ExcludePaths(t *testing.T) {
	validator, _ := setupTestValidator(t)

	handler := Middleware(validator,
		WithExcludePaths("/health", "/ready", "/public/info"),
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		path       string
		wantStatus int
	}{
		{"/health", http.StatusOK},
		{"/ready", http.StatusOK},
		{"/public/info", http.StatusOK},
		{"/api/protected", http.StatusUnauthorized},
		{"/health/deep", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, http.NoBody)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestMiddleware_ExcludePatterns(t *testing.T) {
	validator, _ := setupTestValidator(t)

	handler := Middleware(validator,
		WithExcludePatterns(`^/public/.*`, `^/v[0-9]+/health$`),
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		path       string
		wantStatus int
	}{
		{"/public/docs", http.StatusOK},
		{"/public/anything", http.StatusOK},
		{"/v1/health", http.StatusOK},
		{"/v2/health", http.StatusOK},
		{"/v1/api", http.StatusUnauthorized},
		{"/private/docs", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, http.NoBody)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestMiddleware_InvalidPattern(t *testing.T) {
	validator, rsaKey := setupTestValidator(t)

	// Invalid regex pattern should be silently ignored.
	handler := Middleware(validator,
		WithExcludePatterns(`[invalid`, `/valid`),
		WithExcludePaths("/health"),
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Health path should still work.
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("/health status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Valid pattern should work.
	req = httptest.NewRequest(http.MethodGet, "/valid", http.NoBody)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("/valid status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Protected path with valid token should work.
	claims := &Claims{UserID: "user-123"}
	tokenString := createValidTestToken(t, rsaKey, claims)

	req = httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("/api/test status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestMiddleware_ContextValuesWithEmptyFields(t *testing.T) {
	validator, rsaKey := setupTestValidator(t)

	// Claims with empty optional fields.
	claims := &Claims{
		UserID: "user-456",
		// UserType, Roles, Permissions are empty.
	}
	tokenString := createValidTestToken(t, rsaKey, claims)

	var capturedUserType string
	var capturedRoles []string

	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	// Empty fields should not be set in context.
	if capturedUserType != "" {
		t.Errorf("UserType should be empty, got %q", capturedUserType)
	}

	if len(capturedRoles) != 0 {
		t.Errorf("Roles should be empty, got %v", capturedRoles)
	}
}

func TestMiddleware_BearerCaseInsensitive(t *testing.T) {
	validator, rsaKey := setupTestValidator(t)

	claims := &Claims{UserID: "user-123"}
	tokenString := createValidTestToken(t, rsaKey, claims)

	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

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
			req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
			req.Header.Set("Authorization", tt.prefix+" "+tokenString)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}
		})
	}
}

func TestMiddleware_ContentTypeHeader(t *testing.T) {
	validator, _ := setupTestValidator(t)

	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json; charset=utf-8")
	}

	nosniff := rec.Header().Get("X-Content-Type-Options")
	if nosniff != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want %q", nosniff, "nosniff")
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		wantToken  string
		wantErr    bool
	}{
		{
			name:       "valid bearer token",
			authHeader: "Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9",
			wantToken:  "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9",
			wantErr:    false,
		},
		{
			name:       "lowercase bearer",
			authHeader: "bearer token123",
			wantToken:  "token123",
			wantErr:    false,
		},
		{
			name:       "empty header",
			authHeader: "",
			wantToken:  "",
			wantErr:    true,
		},
		{
			name:       "bearer only",
			authHeader: "Bearer",
			wantToken:  "",
			wantErr:    true,
		},
		{
			name:       "bearer with empty token",
			authHeader: "Bearer ",
			wantToken:  "",
			wantErr:    true,
		},
		{
			name:       "basic auth",
			authHeader: "Basic dXNlcjpwYXNz",
			wantToken:  "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			token, err := extractBearerToken(req)

			if tt.wantErr {
				if err == nil {
					t.Error("extractBearerToken() expected error")
				}
				return
			}

			if err != nil {
				t.Errorf("extractBearerToken() error = %v", err)
				return
			}

			if token != tt.wantToken {
				t.Errorf("extractBearerToken() = %q, want %q", token, tt.wantToken)
			}
		})
	}
}

func TestShouldExclude(t *testing.T) {
	excludeSet := map[string]bool{
		"/health": true,
		"/ready":  true,
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"exact match health", "/health", true},
		{"exact match ready", "/ready", true},
		{"not excluded", "/api/test", false},
		{"partial match", "/health/deep", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldExclude(tt.path, excludeSet, nil)
			if got != tt.want {
				t.Errorf("shouldExclude() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		want       string
	}{
		{
			name:       "X-Forwarded-For single",
			headers:    map[string]string{"X-Forwarded-For": "192.168.1.1"},
			remoteAddr: "10.0.0.1:1234",
			want:       "192.168.1.1",
		},
		{
			name:       "X-Forwarded-For multiple",
			headers:    map[string]string{"X-Forwarded-For": "192.168.1.1, 10.0.0.2, 10.0.0.3"},
			remoteAddr: "10.0.0.1:1234",
			want:       "192.168.1.1",
		},
		{
			name:       "X-Real-IP",
			headers:    map[string]string{"X-Real-IP": "192.168.2.2"},
			remoteAddr: "10.0.0.1:1234",
			want:       "192.168.2.2",
		},
		{
			name:       "X-Forwarded-For takes precedence",
			headers:    map[string]string{"X-Forwarded-For": "192.168.1.1", "X-Real-IP": "192.168.2.2"},
			remoteAddr: "10.0.0.1:1234",
			want:       "192.168.1.1",
		},
		{
			name:       "fallback to RemoteAddr",
			headers:    map[string]string{},
			remoteAddr: "10.0.0.1:1234",
			want:       "10.0.0.1:1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := getClientIP(req)
			if got != tt.want {
				t.Errorf("getClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLogAuthFailure_NilLogger(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)

	// Should not panic with nil logger.
	logAuthFailure(nil, req, "test reason")
}

func TestLogAuthFailure_WithRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	ctx := middleware.WithRequestID(req.Context(), "req-123")
	req = req.WithContext(ctx)

	// Should not panic and should include request ID.
	logAuthFailure(nil, req, "test reason")
}

func TestMiddleware_PreservesRequestContext(t *testing.T) {
	validator, rsaKey := setupTestValidator(t)

	type ctxKey string
	const testKey ctxKey = "test-key"

	claims := &Claims{UserID: "user-123"}
	tokenString := createValidTestToken(t, rsaKey, claims)

	var capturedValue string

	handler := Middleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v, ok := r.Context().Value(testKey).(string); ok {
			capturedValue = v
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	req = req.WithContext(context.WithValue(req.Context(), testKey, "test-value"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedValue != "test-value" {
		t.Errorf("context value = %q, want %q", capturedValue, "test-value")
	}
}
