package middleware

import (
	"net/http"
	"testing"

	"github.com/Dorico-Dynamics/txova-go-core/errors"
)

func TestHTTPStatus(t *testing.T) {
	tests := []struct {
		name string
		code errors.Code
		want int
	}{
		{
			name: "token required returns 401",
			code: CodeTokenRequired,
			want: http.StatusUnauthorized,
		},
		{
			name: "request timeout returns 503",
			code: CodeRequestTimeout,
			want: http.StatusServiceUnavailable,
		},
		{
			name: "maintenance mode returns 503",
			code: CodeMaintenanceMode,
			want: http.StatusServiceUnavailable,
		},
		{
			name: "unknown code returns 500",
			code: errors.Code("UNKNOWN"),
			want: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HTTPStatus(tt.code)
			if got != tt.want {
				t.Errorf("HTTPStatus(%q) = %d, want %d", tt.code, got, tt.want)
			}
		})
	}
}

func TestTokenRequired(t *testing.T) {
	err := TokenRequired()

	if err == nil {
		t.Fatal("TokenRequired() returned nil")
	}

	if err.Code() != CodeTokenRequired {
		t.Errorf("TokenRequired().Code() = %q, want %q", err.Code(), CodeTokenRequired)
	}

	if err.Message() == "" {
		t.Error("TokenRequired().Message() is empty")
	}

	// Use package-level HTTPStatus function for custom codes.
	if HTTPStatus(err.Code()) != http.StatusUnauthorized {
		t.Errorf("HTTPStatus(TokenRequired().Code()) = %d, want %d", HTTPStatus(err.Code()), http.StatusUnauthorized)
	}
}

func TestRequestTimeout(t *testing.T) {
	err := RequestTimeout()

	if err == nil {
		t.Fatal("RequestTimeout() returned nil")
	}

	if err.Code() != CodeRequestTimeout {
		t.Errorf("RequestTimeout().Code() = %q, want %q", err.Code(), CodeRequestTimeout)
	}

	if err.Message() == "" {
		t.Error("RequestTimeout().Message() is empty")
	}

	// Use package-level HTTPStatus function for custom codes.
	if HTTPStatus(err.Code()) != http.StatusServiceUnavailable {
		t.Errorf("HTTPStatus(RequestTimeout().Code()) = %d, want %d", HTTPStatus(err.Code()), http.StatusServiceUnavailable)
	}
}

func TestMaintenanceMode(t *testing.T) {
	tests := []struct {
		name        string
		message     string
		wantDefault bool
	}{
		{
			name:        "with custom message",
			message:     "scheduled maintenance until 5pm",
			wantDefault: false,
		},
		{
			name:        "with empty message uses default",
			message:     "",
			wantDefault: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MaintenanceMode(tt.message)

			if err == nil {
				t.Fatal("MaintenanceMode() returned nil")
			}

			if err.Code() != CodeMaintenanceMode {
				t.Errorf("MaintenanceMode().Code() = %q, want %q", err.Code(), CodeMaintenanceMode)
			}

			if tt.wantDefault {
				if err.Message() == "" {
					t.Error("MaintenanceMode().Message() is empty when default expected")
				}
			} else {
				if err.Message() != tt.message {
					t.Errorf("MaintenanceMode().Message() = %q, want %q", err.Message(), tt.message)
				}
			}

			// Use package-level HTTPStatus function for custom codes.
			if HTTPStatus(err.Code()) != http.StatusServiceUnavailable {
				t.Errorf("HTTPStatus(MaintenanceMode().Code()) = %d, want %d", HTTPStatus(err.Code()), http.StatusServiceUnavailable)
			}
		})
	}
}

func TestIsTokenRequired(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "token required error",
			err:  TokenRequired(),
			want: true,
		},
		{
			name: "different error",
			err:  RequestTimeout(),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTokenRequired(tt.err)
			if got != tt.want {
				t.Errorf("IsTokenRequired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsRequestTimeout(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "request timeout error",
			err:  RequestTimeout(),
			want: true,
		},
		{
			name: "different error",
			err:  TokenRequired(),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRequestTimeout(tt.err)
			if got != tt.want {
				t.Errorf("IsRequestTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsMaintenanceMode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "maintenance mode error",
			err:  MaintenanceMode(""),
			want: true,
		},
		{
			name: "maintenance mode with message",
			err:  MaintenanceMode("custom message"),
			want: true,
		},
		{
			name: "different error",
			err:  TokenRequired(),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsMaintenanceMode(tt.err)
			if got != tt.want {
				t.Errorf("IsMaintenanceMode() = %v, want %v", got, tt.want)
			}
		})
	}
}
