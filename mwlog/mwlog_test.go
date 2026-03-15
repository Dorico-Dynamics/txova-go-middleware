package mwlog

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Dorico-Dynamics/txova-go-core/logging"

	middleware "github.com/Dorico-Dynamics/txova-go-middleware"
)

func newTestLogger() (*logging.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	cfg := logging.Config{
		Level:  slog.LevelDebug,
		Format: logging.FormatText,
		Output: buf,
	}
	logger := logging.New(cfg)
	return logger, buf
}

func TestMiddleware_LogsRequest(t *testing.T) {
	logger, logBuf := newTestLogger()
	mw := Middleware(logger)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	req.Header.Set("User-Agent", "test-agent")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "http request") {
		t.Error("request was not logged")
	}
	if !strings.Contains(logOutput, "GET") {
		t.Error("method was not logged")
	}
	if !strings.Contains(logOutput, "/api/test") {
		t.Error("path was not logged")
	}
	if !strings.Contains(logOutput, "status=200") {
		t.Error("status was not logged")
	}
	if !strings.Contains(logOutput, "test-agent") {
		t.Error("user agent was not logged")
	}
}

func TestMiddleware_LogsStatusLevels(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		wantLevel string
	}{
		{"2xx logs INFO", http.StatusOK, "INFO"},
		{"3xx logs INFO", http.StatusMovedPermanently, "INFO"},
		{"4xx logs WARN", http.StatusNotFound, "WARN"},
		{"5xx logs ERROR", http.StatusInternalServerError, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, logBuf := newTestLogger()
			mw := Middleware(logger)

			handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			logOutput := logBuf.String()
			if !strings.Contains(logOutput, tt.wantLevel) {
				t.Errorf("expected %s level log, got: %s", tt.wantLevel, logOutput)
			}
		})
	}
}

func TestMiddleware_ExcludesPaths(t *testing.T) {
	logger, logBuf := newTestLogger()
	mw := Middleware(logger, WithExcludePaths("/health", "/ready"))

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test excluded path.
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if logBuf.Len() > 0 {
		t.Errorf("excluded path was logged: %s", logBuf.String())
	}

	// Test non-excluded path.
	logBuf.Reset()
	req = httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if logBuf.Len() == 0 {
		t.Error("non-excluded path was not logged")
	}
}

func TestMiddleware_DefaultExcludePaths(t *testing.T) {
	logger, logBuf := newTestLogger()
	mw := Middleware(logger)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	defaultExcluded := []string{"/health", "/ready", "/healthz", "/readyz"}
	for _, path := range defaultExcluded {
		logBuf.Reset()
		req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if logBuf.Len() > 0 {
			t.Errorf("default excluded path %s was logged", path)
		}
	}
}

func TestMiddleware_MasksQueryParams(t *testing.T) {
	logger, logBuf := newTestLogger()
	mw := Middleware(logger, WithMaskQueryParams("token", "secret"))

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test?token=abc123&user=john&secret=xyz", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	if strings.Contains(logOutput, "abc123") {
		t.Error("token value was not masked")
	}
	if strings.Contains(logOutput, "xyz") {
		t.Error("secret value was not masked")
	}
	if !strings.Contains(logOutput, "[REDACTED]") {
		t.Error("masked placeholder not found")
	}
	if !strings.Contains(logOutput, "user=john") {
		t.Error("non-sensitive param was incorrectly masked")
	}
}

func TestMiddleware_DefaultMaskedParams(t *testing.T) {
	logger, logBuf := newTestLogger()
	mw := Middleware(logger)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test?token=secret123&api_key=key456&password=pass789", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	if strings.Contains(logOutput, "secret123") {
		t.Error("token value was not masked by default")
	}
	if strings.Contains(logOutput, "key456") {
		t.Error("api_key value was not masked by default")
	}
	if strings.Contains(logOutput, "pass789") {
		t.Error("password value was not masked by default")
	}
}

func TestMiddleware_SlowRequestThreshold(t *testing.T) {
	logger, logBuf := newTestLogger()
	mw := Middleware(logger, WithSlowRequestThreshold(10*time.Millisecond))

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/slow", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "WARN") {
		t.Error("slow request not logged at WARN level")
	}
	if !strings.Contains(logOutput, "slow=true") {
		t.Error("slow flag not set")
	}
}

func TestMiddleware_CapturesResponseStatus(t *testing.T) {
	logger, logBuf := newTestLogger()
	mw := Middleware(logger)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/create", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "status=201") {
		t.Errorf("status 201 not logged, got: %s", logOutput)
	}
}

func TestMiddleware_CapturesResponseBytes(t *testing.T) {
	logger, logBuf := newTestLogger()
	mw := Middleware(logger)

	responseBody := "Hello, World!"
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(responseBody)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "bytes_out=13") {
		t.Errorf("bytes_out not logged correctly, got: %s", logOutput)
	}
}

func TestMiddleware_ContextValues(t *testing.T) {
	logger, logBuf := newTestLogger()
	mw := Middleware(logger)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	ctx := middleware.WithRequestID(req.Context(), "test-request-id")
	ctx = middleware.WithUserID(ctx, "test-user-id")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "test-request-id") {
		t.Error("request_id not logged")
	}
	if !strings.Contains(logOutput, "test-user-id") {
		t.Error("user_id not logged")
	}
}

func TestMiddleware_ClientIP(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		wantIP  string
	}{
		{
			name:    "X-Forwarded-For single IP",
			headers: map[string]string{"X-Forwarded-For": "192.168.1.1"},
			wantIP:  "192.168.1.1",
		},
		{
			name:    "X-Forwarded-For multiple IPs",
			headers: map[string]string{"X-Forwarded-For": "192.168.1.1, 10.0.0.1, 172.16.0.1"},
			wantIP:  "192.168.1.1",
		},
		{
			name:    "X-Real-IP",
			headers: map[string]string{"X-Real-IP": "192.168.1.2"},
			wantIP:  "192.168.1.2",
		},
		{
			name:    "X-Forwarded-For takes precedence",
			headers: map[string]string{"X-Forwarded-For": "192.168.1.1", "X-Real-IP": "192.168.1.2"},
			wantIP:  "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, logBuf := newTestLogger()
			mw := Middleware(logger)

			handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			logOutput := logBuf.String()
			if !strings.Contains(logOutput, tt.wantIP) {
				t.Errorf("expected IP %s in log, got: %s", tt.wantIP, logOutput)
			}
		})
	}
}

func TestResponseWriter_DefaultStatus(t *testing.T) {
	logger, logBuf := newTestLogger()
	mw := Middleware(logger)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't explicitly call WriteHeader - should default to 200.
		if _, err := w.Write([]byte("OK")); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "status=200") {
		t.Errorf("default status 200 not logged, got: %s", logOutput)
	}
}

func TestResponseWriter_DoubleWriteHeader(t *testing.T) {
	logger, logBuf := newTestLogger()
	mw := Middleware(logger)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.WriteHeader(http.StatusOK) // Should be ignored.
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "status=201") {
		t.Errorf("first status should be logged, got: %s", logOutput)
	}
}

func TestMaskQueryParams(t *testing.T) {
	maskSet := map[string]bool{
		"token":  true,
		"secret": true,
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single sensitive param",
			input:    "token=abc123",
			expected: "token=[REDACTED]",
		},
		{
			name:     "multiple params with sensitive",
			input:    "user=john&token=abc123&page=1",
			expected: "user=john&token=[REDACTED]&page=1",
		},
		{
			name:     "case insensitive",
			input:    "TOKEN=abc123&Secret=xyz",
			expected: "TOKEN=[REDACTED]&Secret=[REDACTED]",
		},
		{
			name:     "empty query",
			input:    "",
			expected: "",
		},
		{
			name:     "no sensitive params",
			input:    "user=john&page=1",
			expected: "user=john&page=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskQueryParams(tt.input, maskSet)
			if got != tt.expected {
				t.Errorf("maskQueryParams(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMiddlewareUsesSharedGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		wantIP     string
	}{
		{
			name:       "from RemoteAddr",
			headers:    nil,
			remoteAddr: "192.168.1.100:12345",
			wantIP:     "192.168.1.100",
		},
		{
			name:       "RemoteAddr without port",
			headers:    nil,
			remoteAddr: "192.168.1.100",
			wantIP:     "192.168.1.100",
		},
		{
			name:       "X-Forwarded-For",
			headers:    map[string]string{"X-Forwarded-For": "10.0.0.1"},
			remoteAddr: "192.168.1.100:12345",
			wantIP:     "10.0.0.1",
		},
		{
			name:       "X-Real-IP",
			headers:    map[string]string{"X-Real-IP": "10.0.0.2"},
			remoteAddr: "192.168.1.100:12345",
			wantIP:     "10.0.0.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := middleware.GetClientIP(req)
			if got != tt.wantIP {
				t.Errorf("GetClientIP() = %q, want %q", got, tt.wantIP)
			}
		})
	}
}

func TestWithExcludePaths(t *testing.T) {
	cfg := &Config{}
	opt := WithExcludePaths("/a", "/b")
	opt(cfg)

	if len(cfg.ExcludePaths) != 2 {
		t.Errorf("WithExcludePaths did not set paths correctly")
	}
}

func TestWithMaskQueryParams(t *testing.T) {
	cfg := &Config{}
	opt := WithMaskQueryParams("a", "b")
	opt(cfg)

	if len(cfg.MaskQueryParams) != 2 {
		t.Errorf("WithMaskQueryParams did not set params correctly")
	}
}

func TestWithSlowRequestThreshold(t *testing.T) {
	cfg := &Config{}
	opt := WithSlowRequestThreshold(5 * time.Second)
	opt(cfg)

	if cfg.SlowRequestThreshold != 5*time.Second {
		t.Errorf("WithSlowRequestThreshold did not set threshold correctly")
	}
}

func BenchmarkMiddleware(b *testing.B) {
	logger, _ := newTestLogger()
	mw := Middleware(logger)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}
