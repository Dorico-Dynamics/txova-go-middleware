# txova-go-middleware

HTTP middleware library providing authentication, authorization, rate limiting, logging, and request processing for Chi router.

## Overview

`txova-go-middleware` provides a comprehensive set of HTTP middleware components for Txova services, including JWT authentication, RBAC authorization, distributed rate limiting, request logging, and panic recovery.

**Module:** `github.com/txova/txova-go-middleware`

## Features

- **Authentication** - JWT token validation with claims injection
- **Authorization** - Role-based and permission-based access control
- **Rate Limiting** - Redis-backed distributed rate limiting
- **Request Logging** - Structured request/response logging
- **Panic Recovery** - Graceful panic handling with stack traces
- **Request ID** - Correlation ID injection and propagation
- **CORS** - Configurable cross-origin resource sharing
- **Timeout** - Request timeout enforcement

## Packages

| Package | Description |
|---------|-------------|
| `auth` | JWT authentication middleware |
| `rbac` | Role-based access control |
| `ratelimit` | Distributed rate limiting |
| `logging` | Request logging |
| `recovery` | Panic recovery |
| `requestid` | Request ID handling |
| `cors` | CORS configuration |
| `timeout` | Request timeout |
| `maintenance` | Maintenance mode |

## Installation

```bash
go get github.com/txova/txova-go-middleware
```

## Usage

### Authentication

```go
import "github.com/txova/txova-go-middleware/auth"

r := chi.NewRouter()
r.Use(auth.Middleware(auth.Config{
    PublicKey: publicKey,
    Exclude:   []string{"/health", "/public/*"},
}))

// Access claims in handler
func handler(w http.ResponseWriter, r *http.Request) {
    userID := auth.GetUserID(r.Context())
    userType := auth.GetUserType(r.Context())
}
```

### Role-Based Access Control

```go
import "github.com/txova/txova-go-middleware/rbac"

// Require specific role
r.With(rbac.RequireRole("admin", "support")).Get("/admin/users", handler)

// Require permission
r.With(rbac.RequirePermission("users:read")).Get("/users", handler)

// Require resource owner
r.With(rbac.RequireOwner("user_id")).Put("/users/{user_id}", handler)

// Require user type
r.With(rbac.RequireUserType("driver")).Get("/driver/earnings", handler)
```

### Rate Limiting

```go
import "github.com/txova/txova-go-middleware/ratelimit"

limiter := ratelimit.New(ratelimit.Config{
    Redis:  redisClient,
    Limit:  100,
    Window: time.Minute,
    KeyFunc: ratelimit.ByUser,
})

r.Use(limiter.Middleware())

// Response headers:
// X-RateLimit-Limit: 100
// X-RateLimit-Remaining: 95
// X-RateLimit-Reset: 1609459200
```

### Request Logging

```go
import "github.com/txova/txova-go-middleware/logging"

r.Use(logging.Middleware(logging.Config{
    Logger:      logger,
    ExcludePaths: []string{"/health"},
    MaskParams:  []string{"token", "password"},
}))

// Logs: method, path, status, duration_ms, request_id, user_id, ip
```

### Recommended Middleware Chain

```go
r := chi.NewRouter()

// Order matters!
r.Use(recovery.Middleware())      // 1. Catch panics
r.Use(requestid.Middleware())     // 2. Inject correlation ID
r.Use(logging.Middleware(cfg))    // 3. Log all requests
r.Use(timeout.Middleware(30*time.Second)) // 4. Enforce limits
r.Use(cors.Middleware(corsConfig))        // 5. Handle preflight
r.Use(maintenance.Middleware(mConfig))    // 6. Block if needed
r.Use(ratelimit.Middleware(rlConfig))     // 7. Throttle abuse
r.Use(auth.Middleware(authConfig))        // 8. Verify identity
```

### Panic Recovery

```go
import "github.com/txova/txova-go-middleware/recovery"

r.Use(recovery.Middleware())

// On panic:
// - Logs full stack trace
// - Returns 500 with generic error
// - Never exposes panic details to client
```

### Request Timeout

```go
import "github.com/txova/txova-go-middleware/timeout"

r.Use(timeout.Middleware(timeout.Config{
    Timeout:   30 * time.Second,
    SkipPaths: []string{"/upload"},
}))
```

## Dependencies

**Internal:**
- `txova-go-core`
- `txova-go-security`

**External:**
- `github.com/go-chi/chi/v5` - Router
- `github.com/go-chi/cors` - CORS handling

## Development

### Requirements

- Go 1.25+

### Testing

```bash
go test ./...
```

### Test Coverage Target

> 85%

## License

Proprietary - Dorico Dynamics
