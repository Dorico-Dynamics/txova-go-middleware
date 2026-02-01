# txova-go-middleware

HTTP middleware library providing authentication, authorization, rate limiting, logging, and request processing for Chi router.

## Overview

`txova-go-middleware` provides a comprehensive set of HTTP middleware components for Txova platform services. Built for the Chi router, it offers JWT authentication, RBAC authorization, distributed rate limiting, structured logging, panic recovery, and more.

**Module:** `github.com/Dorico-Dynamics/txova-go-middleware`

## Features

- **Authentication** - JWT token validation with RSA, ECDSA, and HMAC support
- **Authorization** - Role-based and permission-based access control (RBAC)
- **Rate Limiting** - Redis-backed distributed rate limiting with multiple key strategies
- **Request Logging** - Structured request/response logging with PII masking
- **Panic Recovery** - Graceful panic handling with stack traces
- **Request ID** - Correlation ID generation and propagation
- **CORS** - Configurable cross-origin resource sharing
- **Timeout** - Request timeout enforcement with context cancellation
- **Maintenance Mode** - Service maintenance with bypass rules
- **Middleware Chaining** - Composable middleware groups

## Packages

| Package | Description |
|---------|-------------|
| `auth` | JWT authentication with claims extraction |
| `rbac` | Role, permission, and ownership authorization |
| `ratelimit` | Redis-backed distributed rate limiting |
| `mwlog` | Structured HTTP request logging |
| `recovery` | Panic recovery with stack traces |
| `requestid` | Request ID generation and propagation |
| `mwcors` | CORS configuration and handling |
| `timeout` | Request timeout enforcement |
| `maintenance` | Maintenance mode with bypass rules |
| `chain` | Middleware composition utilities |

## Installation

```bash
go get github.com/Dorico-Dynamics/txova-go-middleware
```

## Quick Start

```go
package main

import (
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/Dorico-Dynamics/txova-go-core/logging"
    "github.com/Dorico-Dynamics/txova-go-middleware/auth"
    "github.com/Dorico-Dynamics/txova-go-middleware/mwcors"
    "github.com/Dorico-Dynamics/txova-go-middleware/mwlog"
    "github.com/Dorico-Dynamics/txova-go-middleware/rbac"
    "github.com/Dorico-Dynamics/txova-go-middleware/recovery"
    "github.com/Dorico-Dynamics/txova-go-middleware/requestid"
    "github.com/Dorico-Dynamics/txova-go-middleware/timeout"
)

func main() {
    logger := logging.New()
    
    validator, _ := auth.NewValidator(auth.ValidatorConfig{
        PublicKey: rsaPublicKey,
        Issuer:    "txova-auth",
    })

    r := chi.NewRouter()
    
    // Middleware stack (order matters!)
    r.Use(requestid.Middleware())
    r.Use(recovery.Middleware(logger))
    r.Use(mwlog.Middleware(logger))
    r.Use(mwcors.Middleware(mwcors.WithAllowedOrigins("https://app.txova.com")))
    r.Use(timeout.Middleware(timeout.WithTimeout(30 * time.Second)))
    r.Use(auth.Middleware(validator, auth.WithExcludePaths("/health")))

    // Public routes
    r.Get("/health", healthHandler)

    // Protected routes
    r.Get("/api/profile", profileHandler)
    
    // Admin routes
    r.With(rbac.RequireRole("admin")).Get("/admin/users", adminHandler)

    http.ListenAndServe(":8080", r)
}
```

## Usage Examples

### Authentication

```go
// Create validator
validator, err := auth.NewValidator(auth.ValidatorConfig{
    PublicKey: rsaPublicKey,
    Issuer:    "txova-auth",
    Audience:  []string{"api"},
})

// Required authentication
r.Use(auth.Middleware(validator,
    auth.WithExcludePaths("/health", "/public"),
    auth.WithExcludePatterns(`^/api/v\d+/public/.*`),
))

// Optional authentication (allows unauthenticated)
r.Use(auth.OptionalMiddleware(validator))

// Access claims in handler
func handler(w http.ResponseWriter, r *http.Request) {
    claims, ok := auth.ClaimsFromContext(r.Context())
    if ok {
        userID := claims.UserID
        if claims.HasRole("admin") {
            // Admin logic
        }
    }
}
```

### Role-Based Access Control

```go
// Require role
r.With(rbac.RequireRole("admin")).Get("/admin", adminHandler)

// Require permission
r.With(rbac.RequirePermission("users:write")).Post("/users", createUser)

// Require user type
r.With(rbac.RequireUserType("driver")).Get("/driver/earnings", earnings)

// Require ownership (URL param must match user ID)
r.With(rbac.RequireOwner("userID")).Get("/users/{userID}", getUser)

// Admin OR owner
r.With(rbac.RequireRoleOrOwner("userID", "admin")).Delete("/users/{userID}", deleteUser)
```

### Rate Limiting

```go
limiter := ratelimit.NewLimiter(redisClient,
    ratelimit.WithLimit(100),
    ratelimit.WithWindow(time.Minute),
    ratelimit.WithBurstAllowance(10),
)

// Rate limit by IP
r.Use(ratelimit.Middleware(limiter,
    ratelimit.WithKeyFunc(ratelimit.KeyByIP),
))

// Rate limit by user
r.Use(ratelimit.Middleware(limiter,
    ratelimit.WithKeyFunc(ratelimit.KeyByUser),
))
```

### Request Logging

```go
r.Use(mwlog.Middleware(logger,
    mwlog.WithExcludePaths("/health", "/metrics"),
    mwlog.WithMaskQueryParams("token", "api_key"),
    mwlog.WithSlowRequestThreshold(500 * time.Millisecond),
))
```

### Middleware Chaining

```go
import "github.com/Dorico-Dynamics/txova-go-middleware/chain"

// Create reusable groups
baseGroup := chain.NewGroup(
    requestid.Middleware(),
    recovery.Middleware(logger),
)

authGroup := baseGroup.Extend(auth.Middleware(validator))
adminGroup := authGroup.Extend(rbac.RequireRole("admin"))

r.Handle("/api/profile", authGroup.Then(profileHandler))
r.Handle("/admin/users", adminGroup.Then(adminHandler))
```

## Context Utilities

```go
import middleware "github.com/Dorico-Dynamics/txova-go-middleware"

// Extract values set by middleware
userID, ok := middleware.UserIDFromContext(ctx)
userType, ok := middleware.UserTypeFromContext(ctx)
roles, ok := middleware.RolesFromContext(ctx)
requestID := middleware.RequestIDFromContext(ctx)
```

## Recommended Middleware Order

```go
r.Use(requestid.Middleware())      // 1. Correlation ID
r.Use(recovery.Middleware(logger)) // 2. Catch panics
r.Use(mwlog.Middleware(logger))    // 3. Log requests
r.Use(mwcors.Middleware(...))      // 4. Handle CORS
r.Use(timeout.Middleware(...))     // 5. Enforce timeout
r.Use(maintenance.Middleware(...)) // 6. Block if maintenance
r.Use(ratelimit.Middleware(...))   // 7. Throttle abuse
r.Use(auth.Middleware(...))        // 8. Verify identity
// RBAC middleware on specific routes
```

## Documentation

See [USAGE.md](USAGE.md) for comprehensive documentation with detailed examples.

## Dependencies

### Internal
- `github.com/Dorico-Dynamics/txova-go-core` - Errors and logging
- `github.com/Dorico-Dynamics/txova-go-security` - Audit logging

### External
- `github.com/go-chi/chi/v5` - HTTP router
- `github.com/go-chi/cors` - CORS handling
- `github.com/golang-jwt/jwt/v5` - JWT parsing
- `github.com/google/uuid` - UUID generation

## Development

### Requirements

- Go 1.25+
- Redis (for rate limiting tests)

### Testing

```bash
go test ./...
```

### Linting

```bash
golangci-lint run ./...
```

### Test Coverage

Target: **85%**

```bash
go test ./... -cover
```

## CI/CD

This project uses GitHub Actions for:
- **Testing** - Unit tests with race detection
- **Coverage** - Enforced 85% minimum
- **Linting** - golangci-lint, staticcheck, go vet
- **Security** - gosec scanning
- **SonarQube** - Code quality analysis
- **Releases** - Semantic versioning from conventional commits

## License

Proprietary - Dorico Dynamics
