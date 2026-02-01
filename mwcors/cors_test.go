package mwcors

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddleware_AllowedOrigin(t *testing.T) {
	mw := Middleware(WithAllowedOrigins("https://example.com"))

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", origin, "https://example.com")
	}
}

func TestMiddleware_DisallowedOrigin(t *testing.T) {
	mw := Middleware(WithAllowedOrigins("https://example.com"))

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Origin", "https://malicious.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin != "" {
		t.Errorf("Access-Control-Allow-Origin should be empty for disallowed origin, got %q", origin)
	}
}

func TestMiddleware_WildcardOrigin(t *testing.T) {
	mw := Middleware(WithAllowedOrigins("*"))

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Origin", "https://any-origin.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", origin, "*")
	}
}

func TestMiddleware_PreflightRequest(t *testing.T) {
	mw := Middleware(
		WithAllowedOrigins("https://example.com"),
		WithAllowedMethods("GET", "POST", "PUT"),
		WithAllowedHeaders("Content-Type", "Authorization"),
	)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/test", http.NoBody)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Check allowed methods.
	methods := rec.Header().Get("Access-Control-Allow-Methods")
	if !strings.Contains(methods, "POST") {
		t.Errorf("Access-Control-Allow-Methods should contain POST, got %q", methods)
	}

	// Check allowed headers.
	headers := rec.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(strings.ToLower(headers), "content-type") {
		t.Errorf("Access-Control-Allow-Headers should contain Content-Type, got %q", headers)
	}
}

func TestMiddleware_ExposedHeaders(t *testing.T) {
	mw := Middleware(
		WithAllowedOrigins("https://example.com"),
		WithExposedHeaders("X-Request-ID", "X-Custom-Header"),
	)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	exposed := strings.ToLower(rec.Header().Get("Access-Control-Expose-Headers"))
	if !strings.Contains(exposed, "x-request-id") {
		t.Errorf("Access-Control-Expose-Headers should contain X-Request-ID, got %q", exposed)
	}
	if !strings.Contains(exposed, "x-custom-header") {
		t.Errorf("Access-Control-Expose-Headers should contain X-Custom-Header, got %q", exposed)
	}
}

func TestMiddleware_Credentials(t *testing.T) {
	mw := Middleware(
		WithAllowedOrigins("https://example.com"),
		WithAllowCredentials(true),
	)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	creds := rec.Header().Get("Access-Control-Allow-Credentials")
	if creds != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want %q", creds, "true")
	}
}

func TestMiddleware_NoCredentials(t *testing.T) {
	mw := Middleware(
		WithAllowedOrigins("https://example.com"),
		WithAllowCredentials(false),
	)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	creds := rec.Header().Get("Access-Control-Allow-Credentials")
	if creds == "true" {
		t.Error("Access-Control-Allow-Credentials should not be set when disabled")
	}
}

func TestMiddleware_MaxAge(t *testing.T) {
	mw := Middleware(
		WithAllowedOrigins("https://example.com"),
		WithMaxAge(3600),
	)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/test", http.NoBody)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	maxAge := rec.Header().Get("Access-Control-Max-Age")
	if maxAge != "3600" {
		t.Errorf("Access-Control-Max-Age = %q, want %q", maxAge, "3600")
	}
}

func TestMiddleware_NoOriginHeader(t *testing.T) {
	mw := Middleware(WithAllowedOrigins("https://example.com"))

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	// No Origin header set.
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should proceed normally without CORS headers.
	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin != "" {
		t.Errorf("Access-Control-Allow-Origin should be empty for non-CORS request, got %q", origin)
	}
}

func TestMiddleware_MultipleOrigins(t *testing.T) {
	mw := Middleware(WithAllowedOrigins("https://example.com", "https://another.com"))

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		origin    string
		wantAllow bool
	}{
		{"https://example.com", true},
		{"https://another.com", true},
		{"https://notallowed.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.origin, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			req.Header.Set("Origin", tt.origin)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			origin := rec.Header().Get("Access-Control-Allow-Origin")
			if tt.wantAllow && origin != tt.origin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", origin, tt.origin)
			}
			if !tt.wantAllow && origin != "" {
				t.Errorf("Access-Control-Allow-Origin should be empty, got %q", origin)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if len(cfg.AllowedOrigins) != 0 {
		t.Errorf("DefaultConfig AllowedOrigins should be empty, got %v", cfg.AllowedOrigins)
	}

	if len(cfg.AllowedMethods) == 0 {
		t.Error("DefaultConfig AllowedMethods should not be empty")
	}

	if !cfg.AllowCredentials {
		t.Error("DefaultConfig should allow credentials")
	}

	if cfg.MaxAge != 86400 {
		t.Errorf("DefaultConfig MaxAge = %d, want %d", cfg.MaxAge, 86400)
	}
}

func TestDevelopmentConfig(t *testing.T) {
	cfg := DevelopmentConfig()

	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "*" {
		t.Errorf("DevelopmentConfig AllowedOrigins should be [*], got %v", cfg.AllowedOrigins)
	}
}

func TestMiddlewareWithConfig(t *testing.T) {
	cfg := Config{
		AllowedOrigins:   []string{"https://custom.com"},
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   []string{"X-Custom"},
		ExposedHeaders:   []string{"X-Exposed"},
		MaxAge:           1800,
		AllowCredentials: false,
	}

	mw := MiddlewareWithConfig(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Origin", "https://custom.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://custom.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", origin, "https://custom.com")
	}
}

func TestWithAllowedOrigins(t *testing.T) {
	cfg := &Config{}
	opt := WithAllowedOrigins("https://a.com", "https://b.com")
	opt(cfg)

	if len(cfg.AllowedOrigins) != 2 {
		t.Errorf("WithAllowedOrigins did not set origins correctly")
	}
}

func TestWithAllowedMethods(t *testing.T) {
	cfg := &Config{}
	opt := WithAllowedMethods("GET", "POST")
	opt(cfg)

	if len(cfg.AllowedMethods) != 2 {
		t.Errorf("WithAllowedMethods did not set methods correctly")
	}
}

func TestWithAllowedHeaders(t *testing.T) {
	cfg := &Config{}
	opt := WithAllowedHeaders("X-A", "X-B")
	opt(cfg)

	if len(cfg.AllowedHeaders) != 2 {
		t.Errorf("WithAllowedHeaders did not set headers correctly")
	}
}

func TestWithExposedHeaders(t *testing.T) {
	cfg := &Config{}
	opt := WithExposedHeaders("X-A", "X-B")
	opt(cfg)

	if len(cfg.ExposedHeaders) != 2 {
		t.Errorf("WithExposedHeaders did not set headers correctly")
	}
}

func TestWithMaxAge(t *testing.T) {
	cfg := &Config{}
	opt := WithMaxAge(7200)
	opt(cfg)

	if cfg.MaxAge != 7200 {
		t.Errorf("WithMaxAge did not set max age correctly")
	}
}

func TestWithAllowCredentials(t *testing.T) {
	cfg := &Config{AllowCredentials: true}
	opt := WithAllowCredentials(false)
	opt(cfg)

	if cfg.AllowCredentials {
		t.Errorf("WithAllowCredentials did not set credentials correctly")
	}
}

func BenchmarkMiddleware(b *testing.B) {
	mw := Middleware(WithAllowedOrigins("https://example.com"))

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Origin", "https://example.com")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkMiddleware_Preflight(b *testing.B) {
	mw := Middleware(
		WithAllowedOrigins("https://example.com"),
		WithAllowedMethods("GET", "POST", "PUT", "DELETE"),
	)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/test", http.NoBody)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}
