package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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
			name:       "X-Forwarded-For takes precedence over X-Real-IP",
			xff:        "192.168.1.1",
			xri:        "192.168.2.2",
			remoteAddr: "10.0.0.1:1234",
			want:       "192.168.1.1",
		},
		{
			name:       "fallback to RemoteAddr IPv4 with port",
			remoteAddr: "10.0.0.1:1234",
			want:       "10.0.0.1",
		},
		{
			name:       "RemoteAddr IPv4 without port",
			remoteAddr: "10.0.0.1",
			want:       "10.0.0.1",
		},
		{
			name:       "RemoteAddr IPv6 with port",
			remoteAddr: "[::1]:8080",
			want:       "::1",
		},
		{
			name:       "RemoteAddr IPv6 without port",
			remoteAddr: "[::1]",
			want:       "::1",
		},
		{
			name:       "RemoteAddr full IPv6 with port",
			remoteAddr: "[2001:db8::1]:8080",
			want:       "2001:db8::1",
		},
		{
			name:       "X-Forwarded-For with whitespace",
			xff:        "  192.168.1.1  ",
			remoteAddr: "10.0.0.1:1234",
			want:       "192.168.1.1",
		},
		{
			name:       "X-Real-IP with whitespace",
			xri:        "  192.168.2.2  ",
			remoteAddr: "10.0.0.1:1234",
			want:       "192.168.2.2",
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

			got := GetClientIP(req)
			if got != tt.want {
				t.Errorf("GetClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
