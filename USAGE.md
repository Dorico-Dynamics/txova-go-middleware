# txova-go-middleware Usage Guide

This guide provides comprehensive examples for using the txova-go-middleware library in your Txova services.

## Table of Contents

- [Quick Start](#quick-start)
- [Middleware Packages](#middleware-packages)
  - [Request ID](#request-id)
  - [Recovery](#recovery)
  - [Logging](#logging)
  - [CORS](#cors)
  - [Timeout](#timeout)
  - [Maintenance Mode](#maintenance-mode)
  - [Rate Limiting](#rate-limiting)
  - [Authentication](#authentication)
  - [RBAC Authorization](#rbac-authorization)
- [Middleware Chaining](#middleware-chaining)
- [Context Utilities](#context-utilities)
- [Error Handling](#error-handling)
- [Testing](#testing)

---

## Quick Start

```go
package main

import (
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/Dorico-Dynamics/txova-go-core/logging"
    middleware "github.com/Dorico-Dynamics/txova-go-middleware"
    "github.com/Dorico-Dynamics/txova-go-middleware/auth"
    "github.com/Dorico-Dynamics/txova-go-middleware/chain"
    "github.com/Dorico-Dynamics/txova-go-middleware/mwcors"
    "github.com/Dorico-Dynamics/txova-go-middleware/mwlog"
    "github.com/Dorico-Dynamics/txova-go-middleware/rbac"
    "github.com/Dorico-Dynamics/txova-go-middleware/recovery"
    "github.com/Dorico-Dynamics/txova-go-middleware/requestid"
    "github.com/Dorico-Dynamics/txova-go-middleware/timeout"
)

func main() {
    logger := logging.New()
    
    // Create JWT validator
    validator, _ := auth.NewValidator(auth.ValidatorConfig{
        PublicKey: publicKey,
        Issuer:    "txova-auth",
    })

    r := chi.NewRouter()
    
    // Build middleware stack
    r.Use(requestid.Middleware())
    r.Use(recovery.Middleware(logger))
    r.Use(mwlog.Middleware(logger))
    r.Use(mwcors.Middleware(mwcors.WithAllowedOrigins("https://app.txova.com")))
    r.Use(timeout.Middleware(timeout.WithTimeout(30*time.Second)))
    r.Use(auth.Middleware(validator, auth.WithExcludePaths("/health", "/public")))

    // Public routes
    r.Get("/health", healthHandler)
    r.Get("/public", publicHandler)

    // Protected routes
    r.Route("/api", func(r chi.Router) {
        r.Get("/profile", profileHandler)
        
        // Admin only
        r.With(rbac.RequireRole("admin")).Get("/admin/users", listUsersHandler)
    })

    http.ListenAndServe(":8080", r)
}
```

---

## Middleware Packages

### Request ID

Generates and propagates unique request IDs for distributed tracing.

```go
import "github.com/Dorico-Dynamics/txova-go-middleware/requestid"

// Default: UUID v4 in X-Request-ID header
r.Use(requestid.Middleware())

// Custom header name
r.Use(requestid.Middleware(
    requestid.WithHeaderName("X-Correlation-ID"),
))

// Custom generator
r.Use(requestid.Middleware(
    requestid.WithGenerator(func() string {
        return fmt.Sprintf("req-%d", time.Now().UnixNano())
    }),
))
```

**Behavior:**
- Checks incoming `X-Request-ID` header and preserves if present
- Generates UUID v4 if not present
- Adds request ID to response headers
- Injects into context for downstream use

**Extract in handlers:**

Note: `RequestIDFromContext` is available from the root `middleware` package, not the `requestid` package. The `requestid` package provides the middleware itself, while context utilities are in the root package.

```go
import middleware "github.com/Dorico-Dynamics/txova-go-middleware"

func handler(w http.ResponseWriter, r *http.Request) {
    requestID := middleware.RequestIDFromContext(r.Context())
    logger.Info("processing request", "request_id", requestID)
}
```

---

### Recovery

Recovers from panics and returns safe error responses.

```go
import "github.com/Dorico-Dynamics/txova-go-middleware/recovery"

// Basic usage
r.Use(recovery.Middleware(logger))

// With options
r.Use(recovery.Middleware(logger,
    recovery.WithStackSize(8192),      // Stack trace buffer size
    recovery.WithPrintStack(true),     // Include stack in logs
))
```

**Behavior:**
- Catches panics in handlers
- Logs panic with stack trace (includes request ID and user ID if available)
- Returns generic 500 Internal Server Error (never exposes panic details)

---

### Logging

Structured HTTP request/response logging.

```go
import "github.com/Dorico-Dynamics/txova-go-middleware/mwlog"

// Basic usage
r.Use(mwlog.Middleware(logger))

// With options
r.Use(mwlog.Middleware(logger,
    mwlog.WithExcludePaths("/health", "/metrics", "/ready"),
    mwlog.WithMaskQueryParams("token", "api_key", "secret"),
    mwlog.WithSlowRequestThreshold(500*time.Millisecond),
))
```

**Default excludes:** `/health`, `/ready`, `/healthz`, `/readyz`

**Default masked params:** `token`, `key`, `secret`, `password`, `api_key`, `apikey`

**Log levels:**
- INFO: 2xx and 3xx responses
- WARN: 4xx responses
- ERROR: 5xx responses

**Logged fields:**
- `method`, `path`, `status`, `duration_ms`
- `request_id`, `user_id`, `ip`, `user_agent`, `bytes`

---

### CORS

Cross-Origin Resource Sharing configuration.

```go
import "github.com/Dorico-Dynamics/txova-go-middleware/mwcors"

// Production: specific origins
r.Use(mwcors.Middleware(
    mwcors.WithAllowedOrigins("https://app.txova.com", "https://admin.txova.com"),
    mwcors.WithAllowedMethods("GET", "POST", "PUT", "DELETE", "PATCH"),
    mwcors.WithAllowedHeaders("Authorization", "Content-Type", "X-Request-ID"),
    mwcors.WithExposedHeaders("X-Request-ID", "X-RateLimit-Remaining"),
    mwcors.WithMaxAge(86400),
    mwcors.WithAllowCredentials(true),
))

// Development: permissive (allow all origins)
r.Use(mwcors.MiddlewareWithConfig(mwcors.DevelopmentConfig()))

// Use default config as base
cfg := mwcors.DefaultConfig()
cfg.AllowedOrigins = []string{"https://example.com"}
r.Use(mwcors.MiddlewareWithConfig(cfg))
```

---

### Timeout

Enforces request timeouts and cancels contexts.

```go
import "github.com/Dorico-Dynamics/txova-go-middleware/timeout"

// Default: 30 seconds
r.Use(timeout.Middleware())

// Custom timeout
r.Use(timeout.Middleware(
    timeout.WithTimeout(15*time.Second),
))

// Skip specific paths (e.g., file uploads, long-running operations)
r.Use(timeout.Middleware(
    timeout.WithTimeout(30*time.Second),
    timeout.WithSkipPaths("/upload", "/export", "/long-running"),
))
```

**Behavior:**
- Creates context with deadline
- Returns 503 Service Unavailable on timeout
- Prevents writes after timeout
- Handler context is cancelled on timeout

**Check timeout in handlers:**
```go
func handler(w http.ResponseWriter, r *http.Request) {
    select {
    case <-r.Context().Done():
        // Request timed out or was cancelled
        return
    case result := <-doWork():
        json.NewEncoder(w).Encode(result)
    }
}
```

---

### Maintenance Mode

Blocks requests when service is in maintenance.

```go
import "github.com/Dorico-Dynamics/txova-go-middleware/maintenance"

// Implement the FlagStore interface
type RedisMaintenanceStore struct {
    client *redis.Client
}

func (s *RedisMaintenanceStore) IsEnabled(ctx context.Context) (bool, error) {
    return s.client.Get(ctx, "maintenance:enabled").Bool()
}

func (s *RedisMaintenanceStore) GetMessage(ctx context.Context) (string, error) {
    return s.client.Get(ctx, "maintenance:message").Result()
}

func (s *RedisMaintenanceStore) GetEndTime(ctx context.Context) (*time.Time, error) {
    ts, err := s.client.Get(ctx, "maintenance:end_time").Int64()
    if err != nil {
        return nil, err
    }
    t := time.Unix(ts, 0)
    return &t, nil
}

// Use in middleware
store := &RedisMaintenanceStore{client: redisClient}
r.Use(maintenance.Middleware(store,
    maintenance.WithBypassIPs("10.0.0.1", "192.168.1.100"),  // Admin IPs
    maintenance.WithBypassPaths("/api/status", "/webhooks"), // Critical paths
    maintenance.WithDefaultMessage("Scheduled maintenance in progress"),
))
```

**Note:** `/health` and `/ready` are always bypassed automatically.

**Response on maintenance:**
```json
{
    "error": {
        "code": "MAINTENANCE_MODE",
        "message": "Service is under maintenance"
    }
}
```

---

### Rate Limiting

Redis-backed distributed rate limiting with fixed window counter.

```go
import "github.com/Dorico-Dynamics/txova-go-middleware/ratelimit"

// Create limiter with Redis client
limiter := ratelimit.NewLimiter(redisClient,
    ratelimit.WithLimit(100),              // 100 requests
    ratelimit.WithWindow(time.Minute),     // per minute
    ratelimit.WithBurstAllowance(10),      // allow 10 extra during burst
    ratelimit.WithKeyPrefix("api"),        // Redis key prefix
)

// Basic: rate limit by IP
r.Use(ratelimit.Middleware(limiter,
    ratelimit.WithKeyFunc(ratelimit.KeyByIP),
))

// Rate limit by authenticated user
r.Use(ratelimit.Middleware(limiter,
    ratelimit.WithKeyFunc(ratelimit.KeyByUser),
))

// Rate limit by endpoint
r.Use(ratelimit.Middleware(limiter,
    ratelimit.WithKeyFunc(ratelimit.KeyByEndpoint),
))

// Combined: by user and endpoint
r.Use(ratelimit.Middleware(limiter,
    ratelimit.WithKeyFunc(ratelimit.KeyByUserAndEndpoint),
))

// Skip certain paths
r.Use(ratelimit.Middleware(limiter,
    ratelimit.WithKeyFunc(ratelimit.KeyByIP),
    ratelimit.WithSkipFunc(func(r *http.Request) bool {
        return r.URL.Path == "/health" || r.URL.Path == "/metrics"
    }),
))

// Custom key function
r.Use(ratelimit.Middleware(limiter,
    ratelimit.WithKeyFunc(func(r *http.Request) string {
        // Rate limit by API key
        return r.Header.Get("X-API-Key")
    }),
))
```

**Response headers:**
```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1609459200
```

**On limit exceeded (429 Too Many Requests):**
```http
Retry-After: 45
```

**Programmatic check:**
```go
count, remaining, resetAt, allowed := limiter.Check(ctx, "user:123")
if !allowed {
    // Handle rate limit exceeded
}
```

---

### Authentication

JWT token validation with claims injection.

```go
import "github.com/Dorico-Dynamics/txova-go-middleware/auth"

// Create validator with RSA public key
validator, err := auth.NewValidator(auth.ValidatorConfig{
    PublicKey: rsaPublicKey,      // *rsa.PublicKey
    Issuer:    "txova-auth",      // Optional: validate iss claim
    Audience:  []string{"api"},   // Optional: validate aud claim
})

// Or with ECDSA
validator, err := auth.NewValidator(auth.ValidatorConfig{
    PublicKey: ecdsaPublicKey,    // *ecdsa.PublicKey
})

// Or with HMAC shared secret
validator, err := auth.NewValidator(auth.ValidatorConfig{
    PublicKey: []byte("shared-secret"),
})
```

**Required authentication:**
```go
r.Use(auth.Middleware(validator,
    auth.WithLogger(logger),
    auth.WithAuditLogger(auditLogger),  // Security audit logging
    auth.WithExcludePaths("/health", "/public", "/login"),
    auth.WithExcludePatterns(`^/api/v\d+/public/.*`),  // Regex patterns
))
```

**Optional authentication** (allows unauthenticated requests):
```go
r.Use(auth.OptionalMiddleware(validator))
```

**Access claims in handlers:**
```go
func handler(w http.ResponseWriter, r *http.Request) {
    claims, ok := auth.ClaimsFromContext(r.Context())
    if !ok {
        // No authentication (only with OptionalMiddleware)
        return
    }

    // Access claim fields
    userID := claims.UserID
    userType := claims.UserType
    roles := claims.Roles
    permissions := claims.Permissions

    // Check roles
    if claims.HasRole("admin") {
        // Admin logic
    }

    if claims.HasAnyRole("admin", "moderator") {
        // Admin or moderator logic
    }

    // Check permissions
    if claims.HasPermission("users:write") {
        // Can write users
    }

    if claims.HasAllPermissions("users:read", "users:write") {
        // Has both permissions
    }
}
```

**Claims structure:**
```go
type Claims struct {
    jwt.RegisteredClaims
    UserID      string   `json:"user_id,omitempty"`
    UserType    string   `json:"user_type,omitempty"`
    Roles       []string `json:"roles,omitempty"`
    Permissions []string `json:"permissions,omitempty"`
}
```

---

### RBAC Authorization

Role-based access control for protecting endpoints.

```go
import "github.com/Dorico-Dynamics/txova-go-middleware/rbac"

// Require ANY of the specified roles
r.With(rbac.RequireRole("admin")).Get("/admin", adminHandler)
r.With(rbac.RequireRole("admin", "support")).Get("/tickets", ticketsHandler)

// Require ALL specified permissions
r.With(rbac.RequirePermission("users:read")).Get("/users", listUsers)
r.With(rbac.RequirePermission("users:read", "users:write")).Put("/users/{id}", updateUser)

// Require specific user type
r.With(rbac.RequireUserType("driver")).Get("/driver/earnings", driverEarnings)
r.With(rbac.RequireUserType("rider", "driver")).Get("/trips", listTrips)

// Require resource ownership (user ID must match URL param)
r.With(rbac.RequireOwner("userID")).Get("/users/{userID}/profile", getProfile)
r.With(rbac.RequireOwner("userID")).Put("/users/{userID}/settings", updateSettings)

// Admin OR owner pattern
r.With(rbac.RequireRoleOrOwner("userID", "admin")).Delete("/users/{userID}", deleteUser)
r.With(rbac.RequireRoleOrOwner("userID", "admin", "support")).Get("/users/{userID}/details", getUserDetails)
```

**With logging and audit:**
```go
mw := rbac.RequireRoleWithOptions(
    []string{"admin"},
    rbac.WithLogger(logger),
    rbac.WithAuditLogger(auditLogger),
)
r.With(mw).Delete("/admin/users/{userID}", deleteUserHandler)
```

**Error responses:**
- 401 Unauthorized: No claims in context (not authenticated)
- 403 Forbidden: Insufficient permissions/roles

---

## Middleware Chaining

Use the `chain` package to compose middleware.

```go
import "github.com/Dorico-Dynamics/txova-go-middleware/chain"

// Direct chaining
mw := chain.Chain(
    requestid.Middleware(),
    recovery.Middleware(logger),
    mwlog.Middleware(logger),
)
handler := mw(finalHandler)

// Using groups (reusable)
baseGroup := chain.NewGroup(
    requestid.Middleware(),
    recovery.Middleware(logger),
    mwlog.Middleware(logger),
)

// Extend for authenticated routes
authGroup := baseGroup.Extend(
    auth.Middleware(validator),
)

// Extend for admin routes
adminGroup := authGroup.Extend(
    rbac.RequireRole("admin"),
)

// Apply to handlers
r.Handle("/public", baseGroup.Then(publicHandler))
r.Handle("/api/profile", authGroup.Then(profileHandler))
r.Handle("/admin/users", adminGroup.Then(adminHandler))
```

**Group methods:**
```go
group := chain.NewGroup(mw1, mw2)
group.Use(mw3, mw4)           // Add more middleware (mutates group)
group.Then(handler)           // Apply to http.Handler
group.ThenFunc(handlerFunc)   // Apply to http.HandlerFunc
group.Middleware()            // Convert to single middleware
group.Clone()                 // Create independent copy
group.Extend(mw5, mw6)        // Create new group with additional middleware
```

---

## Context Utilities

The root `middleware` package provides context utilities.

```go
import middleware "github.com/Dorico-Dynamics/txova-go-middleware"

// Set values
ctx = middleware.WithUserID(ctx, "user-123")
ctx = middleware.WithUserType(ctx, "rider")
ctx = middleware.WithRoles(ctx, []string{"user", "premium"})
ctx = middleware.WithRequestID(ctx, "req-abc123")

// Get values
userID, ok := middleware.UserIDFromContext(ctx)
userType, ok := middleware.UserTypeFromContext(ctx)
roles, ok := middleware.RolesFromContext(ctx)
requestID := middleware.RequestIDFromContext(ctx)  // Returns "" if not set
```

---

## Error Handling

The library uses `txova-go-core/errors` for error handling.

```go
import middleware "github.com/Dorico-Dynamics/txova-go-middleware"

// Create middleware-specific errors
err := middleware.TokenRequired()      // 401
err := middleware.RequestTimeout()     // 503
err := middleware.MaintenanceMode("Scheduled maintenance")  // 503

// Check error types
if middleware.IsTokenRequired(err) {
    // Handle missing token
}

if middleware.IsRequestTimeout(err) {
    // Handle timeout
}

if middleware.IsMaintenanceMode(err) {
    // Handle maintenance
}

// Get HTTP status from error code
status := middleware.HTTPStatus(err.Code())
```

---

## Testing

### Testing middleware

```go
func TestMyHandler(t *testing.T) {
    // Create validator with test key
    validator, _ := auth.NewValidator(auth.ValidatorConfig{
        PublicKey: testRSAPublicKey,
    })

    // Create middleware chain
    handler := auth.Middleware(validator)(
        http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims, _ := auth.ClaimsFromContext(r.Context())
            w.WriteHeader(http.StatusOK)
            json.NewEncoder(w).Encode(claims)
        }),
    )

    // Create test request with token
    req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
    req.Header.Set("Authorization", "Bearer "+validToken)
    rec := httptest.NewRecorder()

    handler.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Errorf("got %d, want %d", rec.Code, http.StatusOK)
    }
}
```

### Testing RBAC

```go
func TestRequireRole(t *testing.T) {
    // Create handler with RBAC
    handler := rbac.RequireRole("admin")(
        http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusOK)
        }),
    )

    // Create request with claims in context
    req := httptest.NewRequest(http.MethodGet, "/admin", nil)
    claims := &auth.Claims{
        UserID: "user-123",
        Roles:  []string{"admin"},
    }
    ctx := auth.WithClaims(req.Context(), claims)
    req = req.WithContext(ctx)

    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Errorf("got %d, want %d", rec.Code, http.StatusOK)
    }
}
```

### Mocking Redis for rate limiting

```go
type MockRedisClient struct {
    counts map[string]int64
}

func (m *MockRedisClient) Incr(ctx context.Context, key string) (int64, error) {
    m.counts[key]++
    return m.counts[key], nil
}

func (m *MockRedisClient) Expire(ctx context.Context, key string, exp time.Duration) error {
    return nil
}

func (m *MockRedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
    return 60 * time.Second, nil
}

func TestRateLimiting(t *testing.T) {
    mock := &MockRedisClient{counts: make(map[string]int64)}
    limiter := ratelimit.NewLimiter(mock,
        ratelimit.WithLimit(2),
        ratelimit.WithWindow(time.Minute),
    )

    handler := ratelimit.Middleware(limiter)(
        http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusOK)
        }),
    )

    // First two requests should succeed
    for i := 0; i < 2; i++ {
        req := httptest.NewRequest(http.MethodGet, "/", nil)
        req.RemoteAddr = "192.168.1.1:12345"
        rec := httptest.NewRecorder()
        handler.ServeHTTP(rec, req)
        if rec.Code != http.StatusOK {
            t.Errorf("request %d: got %d, want %d", i+1, rec.Code, http.StatusOK)
        }
    }

    // Third request should be rate limited
    req := httptest.NewRequest(http.MethodGet, "/", nil)
    req.RemoteAddr = "192.168.1.1:12345"
    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, req)
    if rec.Code != http.StatusTooManyRequests {
        t.Errorf("got %d, want %d", rec.Code, http.StatusTooManyRequests)
    }
}
```

---

## Recommended Middleware Order

```go
r := chi.NewRouter()

// 1. Request ID - First, so all logs have correlation ID
r.Use(requestid.Middleware())

// 2. Recovery - Early, to catch panics from any middleware
r.Use(recovery.Middleware(logger))

// 3. Logging - After recovery to log all requests including recovered panics
r.Use(mwlog.Middleware(logger))

// 4. CORS - Before auth to handle preflight requests
r.Use(mwcors.Middleware(...))

// 5. Timeout - Before auth to limit total request time
r.Use(timeout.Middleware(...))

// 6. Maintenance - Before auth to block during maintenance
r.Use(maintenance.Middleware(...))

// 7. Rate Limiting - Before auth to prevent abuse
r.Use(ratelimit.Middleware(...))

// 8. Authentication - Verify identity
r.Use(auth.Middleware(...))

// 9. RBAC - After auth, on specific routes
r.With(rbac.RequireRole("admin")).Route("/admin", ...)
```
