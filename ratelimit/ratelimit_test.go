package ratelimit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	middleware "github.com/Dorico-Dynamics/txova-go-middleware"
)

// mockRedisClient implements RedisClient for testing.
type mockRedisClient struct {
	mu      sync.Mutex
	data    map[string]int64
	expires map[string]time.Duration
}

func newMockRedisClient() *mockRedisClient {
	return &mockRedisClient{
		data:    make(map[string]int64),
		expires: make(map[string]time.Duration),
	}
}

func (m *mockRedisClient) Incr(_ context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key]++
	return m.data[key], nil
}

func (m *mockRedisClient) Expire(_ context.Context, key string, expiration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expires[key] = expiration
	return nil
}

func (m *mockRedisClient) TTL(_ context.Context, key string) (time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if exp, ok := m.expires[key]; ok {
		return exp, nil
	}
	return 0, nil
}

func TestNewLimiter_Defaults(t *testing.T) {
	client := newMockRedisClient()
	limiter := NewLimiter(client)

	if limiter.limit != 100 {
		t.Errorf("limit = %d, want 100", limiter.limit)
	}

	if limiter.window != time.Minute {
		t.Errorf("window = %v, want %v", limiter.window, time.Minute)
	}

	if limiter.keyPrefix != "ratelimit" {
		t.Errorf("keyPrefix = %q, want %q", limiter.keyPrefix, "ratelimit")
	}
}

func TestNewLimiter_WithOptions(t *testing.T) {
	client := newMockRedisClient()
	limiter := NewLimiter(client,
		WithLimit(50),
		WithWindow(time.Hour),
		WithBurstAllowance(10),
		WithKeyPrefix("myapp"),
	)

	if limiter.limit != 60 { // 50 + 10 burst
		t.Errorf("limit = %d, want 60", limiter.limit)
	}

	if limiter.window != time.Hour {
		t.Errorf("window = %v, want %v", limiter.window, time.Hour)
	}

	if limiter.keyPrefix != "myapp" {
		t.Errorf("keyPrefix = %q, want %q", limiter.keyPrefix, "myapp")
	}
}

func TestLimiter_Check_Allowed(t *testing.T) {
	client := newMockRedisClient()
	limiter := NewLimiter(client, WithLimit(10))

	for i := 1; i <= 10; i++ {
		count, remaining, _, allowed := limiter.Check(context.Background(), "test-key")

		if !allowed {
			t.Errorf("request %d: expected allowed", i)
		}

		if int(count) != i {
			t.Errorf("request %d: count = %d, want %d", i, count, i)
		}

		expectedRemaining := 10 - i
		if remaining != expectedRemaining {
			t.Errorf("request %d: remaining = %d, want %d", i, remaining, expectedRemaining)
		}
	}
}

func TestLimiter_Check_RateLimited(t *testing.T) {
	client := newMockRedisClient()
	limiter := NewLimiter(client, WithLimit(5))

	// First 5 requests should be allowed.
	for i := 1; i <= 5; i++ {
		_, _, _, allowed := limiter.Check(context.Background(), "test-key")
		if !allowed {
			t.Errorf("request %d: expected allowed", i)
		}
	}

	// 6th request should be rate limited.
	count, remaining, _, allowed := limiter.Check(context.Background(), "test-key")

	if allowed {
		t.Error("request 6: expected rate limited")
	}

	if count != 6 {
		t.Errorf("count = %d, want 6", count)
	}

	if remaining != 0 {
		t.Errorf("remaining = %d, want 0", remaining)
	}
}

func TestLimiter_Check_ResetTime(t *testing.T) {
	client := newMockRedisClient()
	limiter := NewLimiter(client, WithWindow(time.Minute))

	_, _, resetAt, _ := limiter.Check(context.Background(), "test-key")

	// Reset time should be in the future.
	if resetAt.Before(time.Now()) {
		t.Error("reset time should be in the future")
	}

	// Reset time should be within the window.
	if resetAt.After(time.Now().Add(time.Minute + time.Second)) {
		t.Error("reset time should be within window")
	}
}

func TestMiddleware_UnderLimit(t *testing.T) {
	client := newMockRedisClient()
	limiter := NewLimiter(client, WithLimit(10))

	handler := Middleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.RemoteAddr = "192.168.1.1:1234"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Check rate limit headers.
	if rec.Header().Get("X-RateLimit-Limit") != "10" {
		t.Errorf("X-RateLimit-Limit = %q, want %q", rec.Header().Get("X-RateLimit-Limit"), "10")
	}

	if rec.Header().Get("X-RateLimit-Remaining") != "9" {
		t.Errorf("X-RateLimit-Remaining = %q, want %q", rec.Header().Get("X-RateLimit-Remaining"), "9")
	}

	if rec.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("X-RateLimit-Reset header should be set")
	}
}

func TestMiddleware_AtLimit(t *testing.T) {
	client := newMockRedisClient()
	limiter := NewLimiter(client, WithLimit(3))

	handler := Middleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make 3 allowed requests.
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
		req.RemoteAddr = "192.168.1.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}

	// 4th request should be rate limited.
	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.RemoteAddr = "192.168.1.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}

	// Check Retry-After header.
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header should be set")
	}

	// Check error response.
	var errResp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}

	if errResp.Error.Code != "RATE_LIMITED" {
		t.Errorf("error code = %q, want %q", errResp.Error.Code, "RATE_LIMITED")
	}
}

func TestMiddleware_SkipFunc(t *testing.T) {
	client := newMockRedisClient()
	limiter := NewLimiter(client, WithLimit(1))

	skipPaths := map[string]bool{
		"/health": true,
		"/ready":  true,
	}

	handler := Middleware(limiter, WithSkipFunc(func(r *http.Request) bool {
		return skipPaths[r.URL.Path]
	}))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make multiple requests to health endpoint - should not be rate limited.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
		req.RemoteAddr = "192.168.1.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("/health request %d: status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}

		// Rate limit headers should not be set for skipped requests.
		if rec.Header().Get("X-RateLimit-Limit") != "" {
			t.Error("X-RateLimit-Limit should not be set for skipped requests")
		}
	}
}

func TestMiddleware_KeyByIP(t *testing.T) {
	client := newMockRedisClient()
	limiter := NewLimiter(client, WithLimit(2))

	handler := Middleware(limiter, WithKeyFunc(KeyByIP))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Different IPs should have separate limits.
	for _, ip := range []string{"192.168.1.1:1234", "192.168.1.2:1234"} {
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
			req.RemoteAddr = ip
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("IP %s request %d: status = %d, want %d", ip, i+1, rec.Code, http.StatusOK)
			}
		}
	}
}

func TestMiddleware_KeyByUser(t *testing.T) {
	client := newMockRedisClient()
	limiter := NewLimiter(client, WithLimit(2))

	handler := Middleware(limiter, WithKeyFunc(KeyByUser))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Different users should have separate limits.
	for _, userID := range []string{"user-1", "user-2"} {
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
			ctx := middleware.WithUserID(req.Context(), userID)
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("user %s request %d: status = %d, want %d", userID, i+1, rec.Code, http.StatusOK)
			}
		}
	}
}

func TestMiddleware_KeyByEndpoint(t *testing.T) {
	client := newMockRedisClient()
	limiter := NewLimiter(client, WithLimit(2))

	handler := Middleware(limiter, WithKeyFunc(KeyByEndpoint))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Different endpoints should have separate limits.
	for _, path := range []string{"/api/users", "/api/orders"} {
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("path %s request %d: status = %d, want %d", path, i+1, rec.Code, http.StatusOK)
			}
		}
	}
}

func TestKeyByIPAndEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.RemoteAddr = "192.168.1.1:1234"

	key := KeyByIPAndEndpoint(req)

	if key != "ip:192.168.1.1:path:/api/test" {
		t.Errorf("key = %q, want %q", key, "ip:192.168.1.1:path:/api/test")
	}
}

func TestKeyByUserAndEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	ctx := middleware.WithUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)

	key := KeyByUserAndEndpoint(req)

	if key != "user:user-123:path:/api/test" {
		t.Errorf("key = %q, want %q", key, "user:user-123:path:/api/test")
	}
}

func TestKeyByUserAndEndpoint_NoUser(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.RemoteAddr = "192.168.1.1:1234"

	key := KeyByUserAndEndpoint(req)

	if key != "ip:192.168.1.1:path:/api/test" {
		t.Errorf("key = %q, want %q", key, "ip:192.168.1.1:path:/api/test")
	}
}

func TestKeyByUser_FallbackToIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.RemoteAddr = "192.168.1.1:1234"

	key := KeyByUser(req)

	if key != "ip:192.168.1.1" {
		t.Errorf("key = %q, want %q", key, "ip:192.168.1.1")
	}
}

func TestGetClientIP_XForwardedFor(t *testing.T) {
	tests := []struct {
		name string
		xff  string
		want string
	}{
		{"single IP", "192.168.1.1", "192.168.1.1"},
		{"multiple IPs", "192.168.1.1, 10.0.0.1, 172.16.0.1", "192.168.1.1"},
		{"with spaces", "  192.168.1.1  ", "192.168.1.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.Header.Set("X-Forwarded-For", tt.xff)

			got := getClientIP(req)
			if got != tt.want {
				t.Errorf("getClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-Real-IP", "10.0.0.1")

	got := getClientIP(req)
	if got != "10.0.0.1" {
		t.Errorf("getClientIP() = %q, want %q", got, "10.0.0.1")
	}
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "172.16.0.1:12345"

	got := getClientIP(req)
	if got != "172.16.0.1" {
		t.Errorf("getClientIP() = %q, want %q", got, "172.16.0.1")
	}
}

func TestGetClientIP_RemoteAddrNoPort(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "172.16.0.1"

	got := getClientIP(req)
	if got != "172.16.0.1" {
		t.Errorf("getClientIP() = %q, want %q", got, "172.16.0.1")
	}
}

func TestMiddleware_BurstAllowance(t *testing.T) {
	client := newMockRedisClient()
	limiter := NewLimiter(client, WithLimit(5), WithBurstAllowance(2))

	handler := Middleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Should allow 7 requests (5 + 2 burst).
	for i := 0; i < 7; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
		req.RemoteAddr = "192.168.1.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}

	// 8th request should be rate limited.
	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.RemoteAddr = "192.168.1.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("request 8: status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestMiddleware_RateLimitHeaders(t *testing.T) {
	client := newMockRedisClient()
	limiter := NewLimiter(client, WithLimit(10), WithWindow(time.Hour))

	handler := Middleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.RemoteAddr = "192.168.1.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Check X-RateLimit-Limit.
	limit := rec.Header().Get("X-RateLimit-Limit")
	if limit != "10" {
		t.Errorf("X-RateLimit-Limit = %q, want %q", limit, "10")
	}

	// Check X-RateLimit-Remaining.
	remaining := rec.Header().Get("X-RateLimit-Remaining")
	if remaining != "9" {
		t.Errorf("X-RateLimit-Remaining = %q, want %q", remaining, "9")
	}

	// Check X-RateLimit-Reset is a valid timestamp.
	resetStr := rec.Header().Get("X-RateLimit-Reset")
	resetTS, err := strconv.ParseInt(resetStr, 10, 64)
	if err != nil {
		t.Errorf("X-RateLimit-Reset is not a valid timestamp: %v", err)
	}

	resetTime := time.Unix(resetTS, 0)
	if resetTime.Before(time.Now()) {
		t.Error("X-RateLimit-Reset should be in the future")
	}
}

func TestMiddleware_ContentTypeOnError(t *testing.T) {
	client := newMockRedisClient()
	limiter := NewLimiter(client, WithLimit(0)) // Immediately rate limited.

	handler := Middleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.RemoteAddr = "192.168.1.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json; charset=utf-8")
	}

	nosniff := rec.Header().Get("X-Content-Type-Options")
	if nosniff != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want %q", nosniff, "nosniff")
	}
}

func TestMiddleware_DefaultKeyFunc(t *testing.T) {
	client := newMockRedisClient()
	limiter := NewLimiter(client, WithLimit(10))

	// Don't pass a KeyFunc - should default to KeyByIP.
	handler := Middleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.RemoteAddr = "192.168.1.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
