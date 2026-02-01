package requestid

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware "github.com/Dorico-Dynamics/txova-go-middleware"
)

func TestMiddleware_GeneratesNewID(t *testing.T) {
	mw := Middleware()

	var capturedRequestID string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID = middleware.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedRequestID == "" {
		t.Error("request ID was not generated")
	}

	// Verify UUID format (8-4-4-4-12).
	parts := strings.Split(capturedRequestID, "-")
	if len(parts) != 5 {
		t.Errorf("request ID is not a valid UUID format: %s", capturedRequestID)
	}

	// Verify response header.
	respHeader := rec.Header().Get(DefaultHeaderName)
	if respHeader != capturedRequestID {
		t.Errorf("response header = %q, want %q", respHeader, capturedRequestID)
	}
}

func TestMiddleware_PropagatesExistingID(t *testing.T) {
	mw := Middleware()
	existingID := "existing-request-id-12345"

	var capturedRequestID string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID = middleware.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set(DefaultHeaderName, existingID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedRequestID != existingID {
		t.Errorf("request ID = %q, want %q", capturedRequestID, existingID)
	}

	// Verify response header preserves the existing ID.
	respHeader := rec.Header().Get(DefaultHeaderName)
	if respHeader != existingID {
		t.Errorf("response header = %q, want %q", respHeader, existingID)
	}
}

func TestMiddleware_CustomHeaderName(t *testing.T) {
	customHeader := "X-Correlation-ID"
	mw := Middleware(WithHeaderName(customHeader))
	existingID := "correlation-id-xyz"

	var capturedRequestID string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID = middleware.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set(customHeader, existingID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedRequestID != existingID {
		t.Errorf("request ID = %q, want %q", capturedRequestID, existingID)
	}

	// Verify custom header in response.
	respHeader := rec.Header().Get(customHeader)
	if respHeader != existingID {
		t.Errorf("response header = %q, want %q", respHeader, existingID)
	}

	// Verify default header is not set.
	defaultHeader := rec.Header().Get(DefaultHeaderName)
	if defaultHeader != "" {
		t.Errorf("default header should be empty, got %q", defaultHeader)
	}
}

func TestMiddleware_CustomGenerator(t *testing.T) {
	customID := "custom-generated-id"
	customGenerator := func() string {
		return customID
	}

	mw := Middleware(WithGenerator(customGenerator))

	var capturedRequestID string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID = middleware.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if capturedRequestID != customID {
		t.Errorf("request ID = %q, want %q", capturedRequestID, customID)
	}
}

func TestMiddleware_UniquenessAcrossRequests(t *testing.T) {
	mw := Middleware()

	requestIDs := make(map[string]bool)
	iterations := 100

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < iterations; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		requestID := rec.Header().Get(DefaultHeaderName)
		if requestIDs[requestID] {
			t.Errorf("duplicate request ID generated at iteration %d: %s", i, requestID)
		}
		requestIDs[requestID] = true
	}
}

func TestMiddleware_HeaderAddedBeforeHandlerWrites(t *testing.T) {
	mw := Middleware()

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that the header is already set before we write.
		if w.Header().Get(DefaultHeaderName) == "" {
			t.Error("request ID header not set before handler execution")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
}

func TestDefaultGenerator(t *testing.T) {
	id1 := DefaultGenerator()
	id2 := DefaultGenerator()

	if id1 == "" {
		t.Error("DefaultGenerator() returned empty string")
	}

	if id1 == id2 {
		t.Error("DefaultGenerator() returned duplicate IDs")
	}

	// Verify UUID format.
	parts := strings.Split(id1, "-")
	if len(parts) != 5 {
		t.Errorf("DefaultGenerator() did not return valid UUID format: %s", id1)
	}
}

func TestWithHeaderName(t *testing.T) {
	cfg := &Config{HeaderName: DefaultHeaderName}
	opt := WithHeaderName("Custom-Header")
	opt(cfg)

	if cfg.HeaderName != "Custom-Header" {
		t.Errorf("WithHeaderName() did not set header name, got %q", cfg.HeaderName)
	}
}

func TestWithGenerator(t *testing.T) {
	cfg := &Config{}
	customGen := func() string { return "custom" }
	opt := WithGenerator(customGen)
	opt(cfg)

	if cfg.Generator == nil {
		t.Error("WithGenerator() did not set generator")
	}
	if cfg.Generator() != "custom" {
		t.Errorf("WithGenerator() generator returned %q, want %q", cfg.Generator(), "custom")
	}
}

func BenchmarkMiddleware(b *testing.B) {
	mw := Middleware()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkMiddleware_WithExistingID(b *testing.B) {
	mw := Middleware()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set(DefaultHeaderName, "existing-id")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}
