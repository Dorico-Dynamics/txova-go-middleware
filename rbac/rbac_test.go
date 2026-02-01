package rbac

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	middleware "github.com/Dorico-Dynamics/txova-go-middleware"
	"github.com/Dorico-Dynamics/txova-go-middleware/auth"
)

func createTestClaims(userID, userType string, roles, permissions []string) *auth.Claims {
	return &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID:      userID,
		UserType:    userType,
		Roles:       roles,
		Permissions: permissions,
	}
}

func withClaims(r *http.Request, claims *auth.Claims) *http.Request {
	ctx := auth.WithClaims(r.Context(), claims)
	ctx = middleware.WithUserID(ctx, claims.UserID)
	return r.WithContext(ctx)
}

func TestRequireRole_NoClaims(t *testing.T) {
	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireRole_HasRole(t *testing.T) {
	claims := createTestClaims("user-123", "admin", []string{"admin", "user"}, nil)

	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req = withClaims(req, claims)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequireRole_HasOneOfMultiple(t *testing.T) {
	claims := createTestClaims("user-123", "admin", []string{"user"}, nil)

	handler := RequireRole("admin", "user", "moderator")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req = withClaims(req, claims)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequireRole_MissingRole(t *testing.T) {
	claims := createTestClaims("user-123", "rider", []string{"user"}, nil)

	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req = withClaims(req, claims)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
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

	if errResp.Error.Code != "FORBIDDEN" {
		t.Errorf("error code = %q, want %q", errResp.Error.Code, "FORBIDDEN")
	}
}

func TestRequirePermission_NoClaims(t *testing.T) {
	handler := RequirePermission("read:users")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequirePermission_HasAllPermissions(t *testing.T) {
	claims := createTestClaims("user-123", "admin", nil, []string{"read:users", "write:users", "delete:users"})

	handler := RequirePermission("read:users", "write:users")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req = withClaims(req, claims)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequirePermission_MissingOnePermission(t *testing.T) {
	claims := createTestClaims("user-123", "admin", nil, []string{"read:users"})

	handler := RequirePermission("read:users", "write:users")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req = withClaims(req, claims)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRequireUserType_NoClaims(t *testing.T) {
	handler := RequireUserType("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireUserType_MatchingType(t *testing.T) {
	claims := createTestClaims("user-123", "driver", []string{"user"}, nil)

	handler := RequireUserType("rider", "driver")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req = withClaims(req, claims)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequireUserType_WrongType(t *testing.T) {
	claims := createTestClaims("user-123", "rider", []string{"user"}, nil)

	handler := RequireUserType("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req = withClaims(req, claims)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRequireOwner_NoClaims(t *testing.T) {
	handler := RequireOwner("userID")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/users/123", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireOwner_IsOwner(t *testing.T) {
	claims := createTestClaims("user-123", "rider", []string{"user"}, nil)

	r := chi.NewRouter()
	r.With(RequireOwner("userID")).Get("/api/users/{userID}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/user-123", http.NoBody)
	req = withClaims(req, claims)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequireOwner_NotOwner(t *testing.T) {
	claims := createTestClaims("user-123", "rider", []string{"user"}, nil)

	r := chi.NewRouter()
	r.With(RequireOwner("userID")).Get("/api/users/{userID}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/user-456", http.NoBody)
	req = withClaims(req, claims)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRequireOwner_MissingURLParam(t *testing.T) {
	claims := createTestClaims("user-123", "rider", []string{"user"}, nil)

	handler := RequireOwner("userID")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/users", http.NoBody)
	req = withClaims(req, claims)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRequireRoleOrOwner_NoClaims(t *testing.T) {
	handler := RequireRoleOrOwner("userID", "admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/users/123", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireRoleOrOwner_HasRole(t *testing.T) {
	claims := createTestClaims("user-123", "admin", []string{"admin"}, nil)

	r := chi.NewRouter()
	r.With(RequireRoleOrOwner("userID", "admin")).Get("/api/users/{userID}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/user-456", http.NoBody)
	req = withClaims(req, claims)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequireRoleOrOwner_IsOwner(t *testing.T) {
	claims := createTestClaims("user-123", "rider", []string{"user"}, nil)

	r := chi.NewRouter()
	r.With(RequireRoleOrOwner("userID", "admin")).Get("/api/users/{userID}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/user-123", http.NoBody)
	req = withClaims(req, claims)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequireRoleOrOwner_NeitherRoleNorOwner(t *testing.T) {
	claims := createTestClaims("user-123", "rider", []string{"user"}, nil)

	r := chi.NewRouter()
	r.With(RequireRoleOrOwner("userID", "admin")).Get("/api/users/{userID}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/user-456", http.NoBody)
	req = withClaims(req, claims)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRequireRoleWithOptions_WithLogger(t *testing.T) {
	claims := createTestClaims("user-123", "rider", []string{"user"}, nil)

	// Just test that it doesn't panic with a nil logger.
	handler := RequireRoleWithOptions([]string{"admin"}, WithLogger(nil))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req = withClaims(req, claims)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestLogAccessDenial_NilLoggers(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)

	// Should not panic with nil loggers in config.
	cfg := &Config{}
	logAccessDenial(cfg, req, "test reason")
}

func TestLogAccessDenial_WithRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	ctx := middleware.WithRequestID(context.Background(), "req-123")
	req = req.WithContext(ctx)

	// Should not panic with nil loggers.
	cfg := &Config{}
	logAccessDenial(cfg, req, "test reason")
}

func TestWriteError_ContentType(t *testing.T) {
	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
