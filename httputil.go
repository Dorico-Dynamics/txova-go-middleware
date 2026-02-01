package middleware

import (
	"net"
	"net/http"
	"strings"
)

// GetClientIP extracts the client IP address from the request.
// It checks the following headers in order:
//  1. X-Forwarded-For (first IP in the list)
//  2. X-Real-IP
//  3. RemoteAddr (with port stripped)
//
// Handles both IPv4 and IPv6 addresses correctly.
func GetClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Use net.SplitHostPort to properly handle IPv4 and IPv6 addresses.
	// IPv6 addresses with ports are in the form "[::1]:8080".
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If SplitHostPort fails (e.g., no port), return the address as-is.
		// Trim brackets from bare IPv6 addresses like "[::1]".
		addr := r.RemoteAddr
		if strings.HasPrefix(addr, "[") && strings.HasSuffix(addr, "]") {
			return addr[1 : len(addr)-1]
		}
		return addr
	}

	return host
}
