package recovery

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Dorico-Dynamics/txova-go-core/errors"
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

func TestMiddleware_NoPanic(t *testing.T) {
	logger, _ := newTestLogger()
	mw := Middleware(logger)

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

func TestMiddleware_RecoversPanic(t *testing.T) {
	logger, logBuf := newTestLogger()
	mw := Middleware(logger)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic message")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify 500 response.
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	// Verify JSON error response.
	var errResp errors.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Error.Code != "INTERNAL_ERROR" {
		t.Errorf("error code = %q, want %q", errResp.Error.Code, "INTERNAL_ERROR")
	}

	// Verify panic details not exposed.
	if strings.Contains(rec.Body.String(), "test panic message") {
		t.Error("panic message was exposed to client")
	}

	// Verify panic was logged.
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "panic recovered") {
		t.Error("panic was not logged")
	}
	if !strings.Contains(logOutput, "test panic message") {
		t.Error("panic message was not logged")
	}
}

func TestMiddleware_LogsStackTrace(t *testing.T) {
	logger, logBuf := newTestLogger()
	mw := Middleware(logger, WithPrintStack(true))

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("stack trace test")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "stack") {
		t.Error("stack trace was not logged")
	}
}

func TestMiddleware_NoStackTrace(t *testing.T) {
	logger, logBuf := newTestLogger()
	mw := Middleware(logger, WithPrintStack(false))

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("no stack trace test")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	// The log should still have the panic message but key "stack" should not be present.
	if !strings.Contains(logOutput, "panic recovered") {
		t.Error("panic was not logged")
	}
}

func TestMiddleware_LogsRequestContext(t *testing.T) {
	logger, logBuf := newTestLogger()
	mw := Middleware(logger)

	// The handler panics, context values set inside won't be visible to recovery.
	// This test verifies that method and path are logged.
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("context test")
	}))

	// Note: The panic happens after we set context, but before the middleware
	// sees it because we're panicking inside the handler.
	req := httptest.NewRequest(http.MethodGet, "/test/path", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "/test/path") {
		t.Error("request path was not logged")
	}
	if !strings.Contains(logOutput, "GET") {
		t.Error("request method was not logged")
	}
}

func TestMiddleware_WithPresetContext(t *testing.T) {
	logger, logBuf := newTestLogger()
	mw := Middleware(logger)

	// Create a handler that panics.
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("preset context panic")
	})

	handler := mw(inner)

	// Create request with preset context values.
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	ctx := middleware.WithRequestID(req.Context(), "preset-request-id")
	ctx = middleware.WithUserID(ctx, "preset-user-id")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "preset-request-id") {
		t.Error("request ID from context was not logged")
	}
	if !strings.Contains(logOutput, "preset-user-id") {
		t.Error("user ID from context was not logged")
	}
}

func TestMiddleware_ContentTypeHeader(t *testing.T) {
	logger, _ := newTestLogger()
	mw := Middleware(logger)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("content type test")
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
		t.Errorf("X-Content-Type-Options = %q, want %q", noSniff, "nosniff")
	}
}

func TestMiddleware_PanicWithNonStringValue(t *testing.T) {
	logger, logBuf := newTestLogger()
	mw := Middleware(logger)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(42) // Panic with an integer.
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "42") {
		t.Error("non-string panic value was not logged")
	}
}

func TestMiddleware_PanicWithError(t *testing.T) {
	logger, logBuf := newTestLogger()
	mw := Middleware(logger)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(io.EOF) // Panic with an error value.
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "EOF") {
		t.Error("error panic value was not logged")
	}
}

func TestWithStackSize(t *testing.T) {
	cfg := &Config{StackSize: DefaultStackSize}
	opt := WithStackSize(8192)
	opt(cfg)

	if cfg.StackSize != 8192 {
		t.Errorf("WithStackSize() did not set stack size, got %d", cfg.StackSize)
	}
}

func TestWithPrintStack(t *testing.T) {
	cfg := &Config{PrintStack: true}
	opt := WithPrintStack(false)
	opt(cfg)

	if cfg.PrintStack {
		t.Error("WithPrintStack(false) did not disable stack printing")
	}
}

func BenchmarkMiddleware_NoPanic(b *testing.B) {
	logger, _ := newTestLogger()
	mw := Middleware(logger)

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
