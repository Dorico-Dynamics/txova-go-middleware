package maintenance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockFlagStore implements FlagStore for testing.
type mockFlagStore struct {
	enabled bool
	message string
	endTime *time.Time
	err     error
}

func (m *mockFlagStore) IsEnabled(_ context.Context) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.enabled, nil
}

func (m *mockFlagStore) GetMessage(_ context.Context) (string, error) {
	return m.message, nil
}

func (m *mockFlagStore) GetEndTime(_ context.Context) (*time.Time, error) {
	return m.endTime, nil
}

func TestMiddleware_MaintenanceOff(t *testing.T) {
	store := &mockFlagStore{enabled: false}

	handlerCalled := false
	handler := Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Error("handler should be called when maintenance is off")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestMiddleware_MaintenanceOn(t *testing.T) {
	store := &mockFlagStore{enabled: true}

	handlerCalled := false
	handler := Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if handlerCalled {
		t.Error("handler should not be called when maintenance is on")
	}

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	// Check response body.
	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Error.Code != "SERVICE_UNAVAILABLE" {
		t.Errorf("error code = %q, want %q", resp.Error.Code, "SERVICE_UNAVAILABLE")
	}
}

func TestMiddleware_BypassPath(t *testing.T) {
	store := &mockFlagStore{enabled: true}

	handlerCalled := false
	handler := Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	// Test default bypass paths.
	bypassPaths := []string{"/health", "/ready"}

	for _, path := range bypassPaths {
		handlerCalled = false
		req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if !handlerCalled {
			t.Errorf("path %s: handler should be called for bypass path", path)
		}

		if rec.Code != http.StatusOK {
			t.Errorf("path %s: status = %d, want %d", path, rec.Code, http.StatusOK)
		}
	}
}

func TestMiddleware_CustomBypassPath(t *testing.T) {
	store := &mockFlagStore{enabled: true}

	handlerCalled := false
	handler := Middleware(store, WithBypassPaths("/admin", "/status"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	// Test custom bypass paths.
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Error("handler should be called for custom bypass path")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestMiddleware_BypassIP(t *testing.T) {
	store := &mockFlagStore{enabled: true}

	handlerCalled := false
	handler := Middleware(store, WithBypassIPs("192.168.1.1", "10.0.0.1"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Error("handler should be called for bypass IP")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestMiddleware_BypassIPFromXForwardedFor(t *testing.T) {
	store := &mockFlagStore{enabled: true}

	handlerCalled := false
	handler := Middleware(store, WithBypassIPs("192.168.1.1"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Error("handler should be called for bypass IP from X-Forwarded-For")
	}
}

func TestMiddleware_NonBypassIP(t *testing.T) {
	store := &mockFlagStore{enabled: true}

	handlerCalled := false
	handler := Middleware(store, WithBypassIPs("192.168.1.1"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.RemoteAddr = "10.0.0.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if handlerCalled {
		t.Error("handler should not be called for non-bypass IP")
	}

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestMiddleware_CustomMessage(t *testing.T) {
	store := &mockFlagStore{
		enabled: true,
		message: "We're updating our systems!",
	}

	handler := Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Error.Message != "We're updating our systems!" {
		t.Errorf("message = %q, want %q", resp.Error.Message, "We're updating our systems!")
	}
}

func TestMiddleware_DefaultMessage(t *testing.T) {
	store := &mockFlagStore{
		enabled: true,
		message: "", // Empty message should use default.
	}

	handler := Middleware(store, WithDefaultMessage("Custom default message"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Error.Message != "Custom default message" {
		t.Errorf("message = %q, want %q", resp.Error.Message, "Custom default message")
	}
}

func TestMiddleware_EndTime(t *testing.T) {
	endTime := time.Now().Add(2 * time.Hour)
	store := &mockFlagStore{
		enabled: true,
		endTime: &endTime,
	}

	handler := Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.EndTime == nil {
		t.Error("end_time should be included in response")
	}
}

func TestMiddleware_NoEndTime(t *testing.T) {
	store := &mockFlagStore{
		enabled: true,
		endTime: nil,
	}

	handler := Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.EndTime != nil {
		t.Error("end_time should not be included when nil")
	}
}

func TestMiddleware_StoreError(t *testing.T) {
	store := &mockFlagStore{
		err: context.DeadlineExceeded,
	}

	handlerCalled := false
	handler := Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// On error, should allow request through.
	if !handlerCalled {
		t.Error("handler should be called when store returns error")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestMiddleware_Headers(t *testing.T) {
	store := &mockFlagStore{enabled: true}

	handler := Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	retryAfter := rec.Header().Get("Retry-After")
	if retryAfter != "300" {
		t.Errorf("Retry-After = %q, want %q", retryAfter, "300")
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		xff        string
		xri        string
		remoteAddr string
		want       string
	}{
		{
			name:       "X-Forwarded-For single",
			xff:        "192.168.1.1",
			remoteAddr: "10.0.0.1:1234",
			want:       "192.168.1.1",
		},
		{
			name:       "X-Forwarded-For multiple",
			xff:        "192.168.1.1, 10.0.0.2, 10.0.0.3",
			remoteAddr: "10.0.0.1:1234",
			want:       "192.168.1.1",
		},
		{
			name:       "X-Real-IP",
			xri:        "192.168.2.2",
			remoteAddr: "10.0.0.1:1234",
			want:       "192.168.2.2",
		},
		{
			name:       "X-Forwarded-For takes precedence",
			xff:        "192.168.1.1",
			xri:        "192.168.2.2",
			remoteAddr: "10.0.0.1:1234",
			want:       "192.168.1.1",
		},
		{
			name:       "fallback to RemoteAddr",
			remoteAddr: "10.0.0.1:1234",
			want:       "10.0.0.1",
		},
		{
			name:       "RemoteAddr without port",
			remoteAddr: "10.0.0.1",
			want:       "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}

			got := getClientIP(req)
			if got != tt.want {
				t.Errorf("getClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
