package chain

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChain_Empty(t *testing.T) {
	chain := Chain()

	handler := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != "OK" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "OK")
	}
}

func TestChain_Single(t *testing.T) {
	addHeader := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "value")
			next.ServeHTTP(w, r)
		})
	}

	chain := Chain(addHeader)

	handler := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Test") != "value" {
		t.Errorf("X-Test = %q, want %q", rec.Header().Get("X-Test"), "value")
	}
}

func TestChain_Order(t *testing.T) {
	var order []string

	first := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "first-before")
			next.ServeHTTP(w, r)
			order = append(order, "first-after")
		})
	}

	second := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "second-before")
			next.ServeHTTP(w, r)
			order = append(order, "second-after")
		})
	}

	third := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "third-before")
			next.ServeHTTP(w, r)
			order = append(order, "third-after")
		})
	}

	chain := Chain(first, second, third)

	handler := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	expected := []string{
		"first-before",
		"second-before",
		"third-before",
		"handler",
		"third-after",
		"second-after",
		"first-after",
	}

	if len(order) != len(expected) {
		t.Fatalf("order length = %d, want %d", len(order), len(expected))
	}

	for i, v := range expected {
		if order[i] != v {
			t.Errorf("order[%d] = %q, want %q", i, order[i], v)
		}
	}
}

func TestChain_MiddlewareCanAbort(t *testing.T) {
	abort := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			// Don't call next
		})
	}

	handlerCalled := false
	chain := Chain(abort)

	handler := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if handlerCalled {
		t.Error("handler should not be called when middleware aborts")
	}

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestNewGroup(t *testing.T) {
	addHeader := func(name, value string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set(name, value)
				next.ServeHTTP(w, r)
			})
		}
	}

	group := NewGroup(
		addHeader("X-First", "1"),
		addHeader("X-Second", "2"),
	)

	handler := group.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-First") != "1" {
		t.Errorf("X-First = %q, want %q", rec.Header().Get("X-First"), "1")
	}

	if rec.Header().Get("X-Second") != "2" {
		t.Errorf("X-Second = %q, want %q", rec.Header().Get("X-Second"), "2")
	}
}

func TestGroup_Use(t *testing.T) {
	addHeader := func(name, value string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set(name, value)
				next.ServeHTTP(w, r)
			})
		}
	}

	group := NewGroup(addHeader("X-First", "1"))
	group.Use(addHeader("X-Second", "2"))
	group.Use(addHeader("X-Third", "3"))

	handler := group.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-First") != "1" {
		t.Errorf("X-First = %q, want %q", rec.Header().Get("X-First"), "1")
	}

	if rec.Header().Get("X-Second") != "2" {
		t.Errorf("X-Second = %q, want %q", rec.Header().Get("X-Second"), "2")
	}

	if rec.Header().Get("X-Third") != "3" {
		t.Errorf("X-Third = %q, want %q", rec.Header().Get("X-Third"), "3")
	}
}

func TestGroup_ThenFunc(t *testing.T) {
	group := NewGroup()

	handler := group.ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ThenFunc"))
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Body.String() != "ThenFunc" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "ThenFunc")
	}
}

func TestGroup_Middleware(t *testing.T) {
	addHeader := func(name, value string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set(name, value)
				next.ServeHTTP(w, r)
			})
		}
	}

	group := NewGroup(addHeader("X-Group", "test"))

	// Use group as middleware.
	mw := group.Middleware()

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Group") != "test" {
		t.Errorf("X-Group = %q, want %q", rec.Header().Get("X-Group"), "test")
	}
}

func TestGroup_Clone(t *testing.T) {
	addHeader := func(name, value string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set(name, value)
				next.ServeHTTP(w, r)
			})
		}
	}

	original := NewGroup(addHeader("X-Original", "1"))
	cloned := original.Clone()

	// Modify original.
	original.Use(addHeader("X-Added", "2"))

	// Cloned should not have the added middleware.
	handler := cloned.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Original") != "1" {
		t.Errorf("X-Original = %q, want %q", rec.Header().Get("X-Original"), "1")
	}

	if rec.Header().Get("X-Added") != "" {
		t.Error("X-Added should not be set on cloned group")
	}
}

func TestGroup_Extend(t *testing.T) {
	addHeader := func(name, value string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set(name, value)
				next.ServeHTTP(w, r)
			})
		}
	}

	base := NewGroup(addHeader("X-Base", "1"))
	extended := base.Extend(addHeader("X-Extended", "2"))

	// Original should not have extended middleware.
	baseHandler := base.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	baseHandler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Extended") != "" {
		t.Error("X-Extended should not be set on base group")
	}

	// Extended should have both.
	extendedHandler := extended.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec = httptest.NewRecorder()

	extendedHandler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Base") != "1" {
		t.Errorf("X-Base = %q, want %q", rec.Header().Get("X-Base"), "1")
	}

	if rec.Header().Get("X-Extended") != "2" {
		t.Errorf("X-Extended = %q, want %q", rec.Header().Get("X-Extended"), "2")
	}
}

func TestGroup_UseChaining(t *testing.T) {
	addHeader := func(name, value string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set(name, value)
				next.ServeHTTP(w, r)
			})
		}
	}

	// Use chaining.
	group := NewGroup().
		Use(addHeader("X-First", "1")).
		Use(addHeader("X-Second", "2"))

	handler := group.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-First") != "1" {
		t.Errorf("X-First = %q, want %q", rec.Header().Get("X-First"), "1")
	}

	if rec.Header().Get("X-Second") != "2" {
		t.Errorf("X-Second = %q, want %q", rec.Header().Get("X-Second"), "2")
	}
}

func TestGroup_Empty(t *testing.T) {
	group := NewGroup()

	handler := group.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("empty group"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != "empty group" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "empty group")
	}
}
