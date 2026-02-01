package timeout

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMiddleware_NormalRequest(t *testing.T) {
	mw := Middleware(WithTimeout(1 * time.Second))

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != "OK" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "OK")
	}
}

func TestMiddleware_Timeout(t *testing.T) {
	mw := Middleware(WithTimeout(50 * time.Millisecond))

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow handler.
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	// Verify error response contains timeout error.
	body := rec.Body.String()
	if !strings.Contains(body, "REQUEST_TIMEOUT") {
		t.Errorf("body should contain REQUEST_TIMEOUT, got: %s", body)
	}
}

func TestMiddleware_ContextCancelled(t *testing.T) {
	mw := Middleware(WithTimeout(50 * time.Millisecond))

	ctxErrChan := make(chan error, 1)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wait for context cancellation.
		<-r.Context().Done()
		ctxErrChan <- r.Context().Err()
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Wait for the handler goroutine to send the error.
	select {
	case ctxErr := <-ctxErrChan:
		if ctxErr != context.DeadlineExceeded {
			t.Errorf("context error = %v, want %v", ctxErr, context.DeadlineExceeded)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for context error")
	}
}

func TestMiddleware_SkipPaths(t *testing.T) {
	mw := Middleware(
		WithTimeout(50*time.Millisecond),
		WithSkipPaths("/upload", "/long-running"),
	)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow handler that would normally timeout.
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("completed")); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))

	// Test skipped path completes normally.
	req := httptest.NewRequest(http.MethodGet, "/upload", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != "completed" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "completed")
	}
}

func TestMiddleware_SkipPathsMultiple(t *testing.T) {
	mw := Middleware(
		WithTimeout(10*time.Millisecond),
		WithSkipPaths("/a", "/b", "/c"),
	)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	skipPaths := []string{"/a", "/b", "/c"}
	for _, path := range skipPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("skip path %s: status code = %d, want %d", path, rec.Code, http.StatusOK)
			}
		})
	}
}

func TestMiddleware_NonSkippedPathTimesOut(t *testing.T) {
	mw := Middleware(
		WithTimeout(10*time.Millisecond),
		WithSkipPaths("/upload"),
	)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("non-skipped path should timeout, got status %d", rec.Code)
	}
}

func TestMiddleware_PartialWriteBeforeTimeout(t *testing.T) {
	mw := Middleware(WithTimeout(50 * time.Millisecond))

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write header before timeout.
		w.WriteHeader(http.StatusAccepted)
		if _, err := w.Write([]byte("partial")); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
		// Then sleep past timeout.
		time.Sleep(100 * time.Millisecond)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should have the handler's status, not timeout status.
	if rec.Code != http.StatusAccepted {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusAccepted)
	}

	if rec.Body.String() != "partial" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "partial")
	}
}

func TestMiddleware_DefaultTimeout(t *testing.T) {
	mw := Middleware()

	// Just verify it doesn't panic with default config.
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestMiddleware_ContentTypeHeader(t *testing.T) {
	mw := Middleware(WithTimeout(10 * time.Millisecond))

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}

	noSniff := rec.Header().Get("X-Content-Type-Options")
	if noSniff != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", noSniff)
	}
}

func TestTimeoutWriter_PreventDoubleWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	tw := &timeoutWriter{ResponseWriter: rec}

	tw.WriteHeader(http.StatusCreated)
	tw.WriteHeader(http.StatusOK) // Should be ignored.

	if rec.Code != http.StatusCreated {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestTimeoutWriter_WriteAfterTimeout(t *testing.T) {
	rec := httptest.NewRecorder()
	tw := &timeoutWriter{ResponseWriter: rec}

	tw.setTimedOut()

	n, err := tw.Write([]byte("should not write"))
	if err != context.DeadlineExceeded {
		t.Errorf("Write after timeout should return DeadlineExceeded, got %v", err)
	}
	if n != 0 {
		t.Errorf("bytes written = %d, want 0", n)
	}
}

func TestTimeoutWriter_WriteHeaderAfterTimeout(t *testing.T) {
	rec := httptest.NewRecorder()
	tw := &timeoutWriter{ResponseWriter: rec}

	tw.setTimedOut()
	tw.WriteHeader(http.StatusOK)

	// The recorder's default is 200, but we shouldn't have explicitly written it.
	if tw.wroteHeader {
		t.Error("WriteHeader should not set wroteHeader after timeout")
	}
}

func TestTimeoutWriter_ImplicitWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	tw := &timeoutWriter{ResponseWriter: rec}

	_, err := tw.Write([]byte("data"))
	if err != nil {
		t.Errorf("Write error = %v", err)
	}

	if !tw.wroteHeader {
		t.Error("Write should implicitly call WriteHeader")
	}
	if tw.code != http.StatusOK {
		t.Errorf("implicit status = %d, want %d", tw.code, http.StatusOK)
	}
}

func TestWithTimeout(t *testing.T) {
	cfg := &Config{Timeout: DefaultTimeout}
	opt := WithTimeout(5 * time.Second)
	opt(cfg)

	if cfg.Timeout != 5*time.Second {
		t.Errorf("WithTimeout did not set timeout correctly")
	}
}

func TestWithSkipPaths(t *testing.T) {
	cfg := &Config{}
	opt := WithSkipPaths("/a", "/b")
	opt(cfg)

	if len(cfg.SkipPaths) != 2 {
		t.Errorf("WithSkipPaths did not set paths correctly")
	}
}

func BenchmarkMiddleware_NoTimeout(b *testing.B) {
	mw := Middleware(WithTimeout(1 * time.Second))

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
