# txova-go-middleware Execution Plan

## Overview

Implementation plan for the HTTP middleware library providing authentication, authorization, rate limiting, logging, and request processing middleware for Chi router in the Txova platform.

**Target Coverage:** > 85%

---

## Internal Dependencies

### txova-go-core
| Package | Types/Functions Used | Purpose |
|---------|---------------------|---------|
| `errors` | `AppError`, `Code`, error constructors | Structured error handling |
| `errors` | `TokenExpired()`, `TokenInvalid()`, `Forbidden()` | Auth/RBAC errors |
| `errors` | `RateLimited()`, `ServiceUnavailable()` | Rate limit/maintenance errors |
| `errors` | `InternalErrorWrap()` | Wrap internal errors |
| `logging` | `Logger`, `*Context()` methods | Structured logging with context |
| `logging` | Masking functions | PII protection in logs |
| `context` | `RequestID()`, `UserID()`, `WithRequestID()` | Context field management |

### txova-go-security
| Package | Types/Functions Used | Purpose |
|---------|---------------------|---------|
| `token` | `Hash()`, `Compare()` | Token validation helpers |
| `mask` | `Phone()`, `Email()` | PII masking in logs |
| `audit` | `Logger`, event types | Security audit logging |

---

## External Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/go-chi/chi/v5` | v5.2.x | HTTP router |
| `github.com/go-chi/cors` | v1.2.x | CORS middleware base |
| `github.com/golang-jwt/jwt/v5` | v5.2.x | JWT parsing and validation |
| `github.com/google/uuid` | v1.6.x | UUID generation for request IDs |

---

## Progress Summary

| Phase | Status | Commit | Coverage |
|-------|--------|--------|----------|
| Phase 1: Foundation | Complete | `e36d9c5` | 100% |
| Phase 2: Request ID & Recovery | Complete | `9e61504` | 98% |
| Phase 3: Logging Middleware | Complete | `eea6aab` | 89% |
| Phase 4: Timeout Middleware | Complete | `4a91cff` | 98% |
| Phase 5: CORS Middleware | Complete | `72b32cc` | 100% |
| Phase 6: Authentication Middleware | Complete | - | 94.4% |
| Phase 7: RBAC Middleware | Pending | - | - |
| Phase 8: Rate Limiting Middleware | Pending | - | - |
| Phase 9: Maintenance Mode Middleware | Pending | - | - |
| Phase 10: Chain & Integration | Pending | - | - |

**Current Branch:** `week1`

---

## Phase 1: Foundation

### 1.1 Project Setup
- [x] Initialize Go module with `github.com/Dorico-Dynamics/txova-go-middleware`
- [x] Add external dependencies:
  - `github.com/go-chi/chi/v5`
  - `github.com/go-chi/cors`
  - `github.com/golang-jwt/jwt/v5`
  - `github.com/google/uuid`
- [x] Add internal dependencies:
  - `github.com/Dorico-Dynamics/txova-go-core`
  - `github.com/Dorico-Dynamics/txova-go-security`
- [x] Create package structure: `auth/`, `rbac/`, `ratelimit/`, `mwlog/`, `requestid/`, `timeout/`, `recovery/`, `mwcors/`, `maintenance/`
- [x] Set up `.golangci.yml` for linting (copy from txova-go-security)

### 1.2 Common Error Types
- [x] Define middleware-specific error codes extending `txova-go-core/errors`:
  - `CodeTokenRequired` - No authorization header provided
  - `CodeRequestTimeout` - Request exceeded timeout
  - `CodeMaintenanceMode` - Service in maintenance mode
- [x] Create error constructors for each middleware error type
- [x] Support `errors.Is()` and `errors.As()` via `txova-go-core/errors` patterns

### 1.3 Context Keys and Helpers
- [x] Define context key types for type-safe context access:
  - `ContextKeyUserID`
  - `ContextKeyUserType`
  - `ContextKeyRoles`
  - `ContextKeyClaims`
  - `ContextKeyRequestID`
- [x] Implement context getter functions:
  - `UserIDFromContext(ctx) (string, bool)`
  - `UserTypeFromContext(ctx) (string, bool)`
  - `RolesFromContext(ctx) ([]string, bool)`
  - `ClaimsFromContext(ctx) (*Claims, bool)`
  - `RequestIDFromContext(ctx) string`
- [x] Implement context setter functions for injecting values

### 1.4 Tests
- [x] Test error codes and HTTP status mappings
- [x] Test context getters/setters with valid and missing values

---

## Phase 2: Request ID & Recovery (`requestid`, `recovery` packages)

### 2.1 Request ID Middleware
- [x] Implement `Middleware() func(http.Handler) http.Handler`
- [x] Check for existing `X-Request-ID` header
- [x] If present, use existing ID (for distributed tracing)
- [x] If absent, generate new UUID v4
- [x] Inject request ID into context using `ContextKeyRequestID`
- [x] Add `X-Request-ID` to response headers
- [x] Implement `Config` struct with options:
  - `HeaderName string` (default: "X-Request-ID")
  - `Generator func() string` (default: UUID v4)

### 2.2 Recovery Middleware
- [x] Implement `Middleware(logger *logging.Logger) func(http.Handler) http.Handler`
- [x] Wrap handler in defer/recover
- [x] On panic:
  - Log full stack trace at ERROR level
  - Include request_id, method, path in log
  - Return 500 + INTERNAL_ERROR response
- [x] Never expose panic details to client
- [x] Implement `Config` struct with options:
  - `StackSize int` (default: 4096)
  - `PrintStack bool` (default: true in dev)

### 2.3 Tests
- [x] Test request ID propagation (existing header)
- [x] Test request ID generation (no header)
- [x] Test request ID in response headers
- [x] Test custom header name
- [x] Test panic recovery returns 500
- [x] Test panic logging includes stack trace
- [x] Test panic details not exposed to client

---

## Phase 3: Logging Middleware (`mwlog` package)

### 3.1 Request Logging
- [x] Implement `Middleware(logger *logging.Logger) func(http.Handler) http.Handler`
- [x] Wrap `http.ResponseWriter` to capture status code and bytes written
- [x] Log at request completion with fields:
  - `method` - HTTP method
  - `path` - Request path (without query params)
  - `status` - Response status code
  - `duration_ms` - Request duration in milliseconds
  - `request_id` - Correlation ID from context
  - `user_id` - Authenticated user (if present)
  - `ip` - Client IP address (respect X-Forwarded-For)
  - `user_agent` - Client user agent
  - `bytes_in` - Request body size
  - `bytes_out` - Response body size

### 3.2 Log Level Selection
- [x] Implement log level based on status:
  - 2xx → INFO
  - 4xx → WARN
  - 5xx → ERROR
- [ ] Include stack trace for 5xx errors (handled by recovery middleware)

### 3.3 Configuration
- [x] Implement `Config` struct:
  - `ExcludePaths []string` - Paths to skip (e.g., /health, /ready)
  - `MaskQueryParams []string` - Params to mask (token, key, secret)
  - `SlowRequestThreshold time.Duration` - Log slow requests at WARN
- [x] Functional options: `WithExcludePaths()`, `WithMaskQueryParams()`

### 3.4 Response Writer Wrapper
- [x] Implement `responseWriter` struct wrapping `http.ResponseWriter`
- [x] Capture status code (default 200 if not set)
- [x] Count bytes written
- [x] Support `http.Flusher`, `http.Hijacker`, `http.Pusher` interfaces

### 3.5 Tests
- [x] Test log fields are populated correctly
- [x] Test log levels based on status codes
- [x] Test path exclusion
- [x] Test query param masking
- [x] Test response writer wrapper captures status
- [x] Test slow request warning
- [x] Test interface support (Flusher, Hijacker)

---

## Phase 4: Timeout Middleware (`timeout` package)

### 4.1 Request Timeout
- [x] Implement `Middleware(timeout time.Duration) func(http.Handler) http.Handler`
- [x] Create context with timeout
- [x] Cancel context when timeout exceeded
- [x] Return 503 + REQUEST_TIMEOUT on timeout

### 4.2 Configuration
- [x] Implement `Config` struct:
  - `Timeout time.Duration` (default: 30s)
  - `SkipPaths []string` - Paths to skip (file uploads)
- [x] Functional options: `WithTimeout()`, `WithSkipPaths()`

### 4.3 Timeout Handling
- [x] Check `ctx.Done()` channel for cancellation
- [x] Allow handler to detect cancellation via `ctx.Err()`
- [x] Write timeout response only if handler hasn't written yet

### 4.4 Tests
- [x] Test normal request completes within timeout
- [x] Test timeout triggers 503 response
- [x] Test context cancellation is propagated
- [x] Test skip paths bypass timeout
- [x] Test partial response handling (already written)

---

## Phase 5: CORS Middleware (`mwcors` package)

### 5.1 CORS Configuration
- [x] Implement `Config` struct:
  - `AllowedOrigins []string` (default: empty - reject all)
  - `AllowedMethods []string` (default: GET, POST, PUT, DELETE, PATCH, OPTIONS)
  - `AllowedHeaders []string` (default: Content-Type, Authorization, X-Request-ID)
  - `ExposedHeaders []string` (default: X-Request-ID)
  - `MaxAge int` (default: 86400 seconds)
  - `AllowCredentials bool` (default: true)

### 5.2 CORS Middleware
- [x] Implement `Middleware(cfg Config) func(http.Handler) http.Handler`
- [x] Wrap `github.com/go-chi/cors` with Txova configuration
- [x] Handle preflight OPTIONS requests
- [x] Validate origin against allowlist
- [x] Set appropriate CORS headers

### 5.3 Origin Validation
- [x] Support exact match origins
- [x] Support wildcard `*` (development only)
- [ ] Log rejected origins at WARN level (handled by go-chi/cors)

### 5.4 Tests
- [x] Test preflight OPTIONS handling
- [x] Test allowed origin passes
- [x] Test rejected origin blocked
- [x] Test credentials header
- [x] Test exposed headers
- [x] Test max age header

---

## Phase 6: Authentication Middleware (`auth` package)

### 6.1 JWT Claims Structure
- [x] Define `Claims` struct extending `jwt.RegisteredClaims`:
  - `UserID string`
  - `UserType string` (rider, driver, admin)
  - `Roles []string`
  - `Permissions []string`
- [x] Implement claims helper methods:
  - `HasRole(role string) bool`
  - `HasAnyRole(roles ...string) bool`
  - `HasPermission(permission string) bool`
  - `HasAllPermissions(permissions ...string) bool`
- [x] Implement `ClaimsFromContext(ctx) (*Claims, bool)`
- [x] Implement `WithClaims(ctx, claims) context.Context`

### 6.2 JWT Validator
- [x] Implement `Validator` struct with configuration:
  - `PublicKey` - RSA/ECDSA public key or HMAC secret for signature verification
  - `Issuer string` - Expected issuer claim
  - `Audience []string` - Expected audience claims
- [x] Implement `NewValidator(cfg ValidatorConfig) (*Validator, error)`
- [x] Implement `Validate(tokenString string) (*Claims, error)`
- [x] Validate signature using configured public key
- [x] Validate expiration time (`exp` claim)
- [x] Validate not-before time (`nbf` claim)
- [x] Validate issuer (`iss` claim)
- [x] Validate audience (`aud` claim)
- [x] Support RSA, ECDSA, and HMAC signing methods

### 6.3 Auth Middleware
- [x] Implement `Middleware(validator *Validator, opts ...Option) func(http.Handler) http.Handler`
- [x] Extract token from "Authorization: Bearer {token}" header
- [x] Return 401 + TOKEN_REQUIRED if no header
- [x] Return 401 + TOKEN_INVALID if malformed
- [x] Validate token using Validator
- [x] Return 401 + TOKEN_EXPIRED if expired
- [x] Return 401 + TOKEN_INVALID if signature invalid
- [x] Inject claims into context on success
- [x] Inject user_id, user_type, roles into context for convenience
- [x] Continue to next handler

### 6.4 Path Exclusion
- [x] Implement `Config` struct:
  - `ExcludePaths []string` - Paths to skip auth (e.g., /health, /login)
  - `ExcludePatterns []string` - Regex patterns to skip
- [x] Functional options: `WithExcludePaths()`, `WithExcludePatterns()`, `WithLogger()`

### 6.5 Logging Integration
- [x] Log auth failures with:
  - IP address (X-Forwarded-For, X-Real-IP support)
  - Request path
  - Failure reason (expired, invalid, missing)
  - Request ID

### 6.6 Tests
- [x] Test missing Authorization header → 401 TOKEN_REQUIRED
- [x] Test invalid format (not "Bearer ...") → 401 TOKEN_INVALID
- [x] Test expired token → 401 TOKEN_EXPIRED
- [x] Test invalid signature → 401 TOKEN_INVALID
- [x] Test valid token → claims in context
- [x] Test path exclusion bypasses auth
- [x] Test pattern exclusion
- [x] Test all signing methods (RS256, RS384, RS512, ES256, HS256, HS384, HS512)
- [x] Test issuer validation
- [x] Test audience validation
- [x] Test context values populated correctly

---

## Phase 7: Optional Auth & RBAC (`auth`, `rbac` packages)

### 7.1 Optional Auth Middleware
- [ ] Implement `OptionalMiddleware(validator *Validator) func(http.Handler) http.Handler`
- [ ] Extract and validate token if present
- [ ] On no token: continue without claims
- [ ] On invalid token: continue without claims (don't error)
- [ ] On valid token: inject claims into context

### 7.2 RBAC Middleware - Role Check
- [ ] Implement `RequireRole(roles ...string) func(http.Handler) http.Handler`
- [ ] Extract claims from context (requires auth middleware first)
- [ ] Check if user has at least one of the specified roles
- [ ] Return 403 + FORBIDDEN if missing required role
- [ ] Log access denial

### 7.3 RBAC Middleware - Permission Check
- [ ] Implement `RequirePermission(permissions ...string) func(http.Handler) http.Handler`
- [ ] Check if user has ALL specified permissions
- [ ] Return 403 + FORBIDDEN if missing any permission

### 7.4 RBAC Middleware - Owner Check
- [ ] Implement `RequireOwner(paramName string) func(http.Handler) http.Handler`
- [ ] Extract user ID from context
- [ ] Extract resource owner ID from URL parameter
- [ ] Compare and allow if match
- [ ] Return 403 + FORBIDDEN if not owner

### 7.5 RBAC Middleware - User Type Check
- [ ] Implement `RequireUserType(types ...string) func(http.Handler) http.Handler`
- [ ] Check if user type matches one of specified types
- [ ] Return 403 + FORBIDDEN if wrong type

### 7.6 Composite Checks
- [ ] Implement `RequireRoleOrOwner(paramName string, roles ...string) func(http.Handler) http.Handler`
- [ ] Allow if user has role OR is owner

### 7.7 Tests
- [ ] Test optional auth with no token → no claims, continues
- [ ] Test optional auth with invalid token → no claims, continues
- [ ] Test optional auth with valid token → claims in context
- [ ] Test RequireRole with matching role → allowed
- [ ] Test RequireRole with missing role → 403
- [ ] Test RequirePermission with all perms → allowed
- [ ] Test RequirePermission with missing perm → 403
- [ ] Test RequireOwner with matching ID → allowed
- [ ] Test RequireOwner with different ID → 403
- [ ] Test RequireUserType with matching type → allowed
- [ ] Test RequireUserType with wrong type → 403
- [ ] Test RequireRoleOrOwner both conditions

---

## Phase 8: Rate Limiting Middleware (`ratelimit` package)

### 8.1 Redis Client Interface
- [ ] Define `RedisClient` interface:
  - `Incr(ctx context.Context, key string) (int64, error)`
  - `Expire(ctx context.Context, key string, expiration time.Duration) error`
  - `TTL(ctx context.Context, key string) (time.Duration, error)`
  - `Get(ctx context.Context, key string) (string, error)`

### 8.2 Rate Limiter
- [ ] Implement `Limiter` struct with configuration:
  - `RedisClient` - Redis client for distributed limiting
  - `Limit int` - Requests per window
  - `Window time.Duration` - Time window
  - `BurstAllowance int` - Extra requests allowed in burst
- [ ] Implement sliding window rate limiting algorithm

### 8.3 Key Strategies
- [ ] Implement `KeyFunc` type: `func(r *http.Request) string`
- [ ] Implement built-in key functions:
  - `KeyByIP(r *http.Request) string` - Rate limit by client IP
  - `KeyByUser(r *http.Request) string` - Rate limit by user ID
  - `KeyByEndpoint(r *http.Request) string` - Rate limit by path
  - `KeyByIPAndEndpoint(r *http.Request) string` - Combined
- [ ] Support custom key functions

### 8.4 Rate Limit Middleware
- [ ] Implement `Middleware(limiter *Limiter, keyFunc KeyFunc) func(http.Handler) http.Handler`
- [ ] Check rate limit for request key
- [ ] Add response headers:
  - `X-RateLimit-Limit` - Max requests
  - `X-RateLimit-Remaining` - Remaining requests
  - `X-RateLimit-Reset` - Reset timestamp (Unix)
- [ ] On under limit: add headers, continue
- [ ] On at limit: return 429 + RATE_LIMITED

### 8.5 Skip Function
- [ ] Implement `SkipFunc` type: `func(r *http.Request) bool`
- [ ] Support bypass for certain requests (e.g., health checks, admins)

### 8.6 Configuration
- [ ] Implement `Config` struct:
  - `Limit int`
  - `Window time.Duration`
  - `BurstAllowance int`
  - `KeyFunc KeyFunc`
  - `SkipFunc SkipFunc`
  - `KeyPrefix string` (default: "ratelimit")
- [ ] Functional options

### 8.7 Tests
- [ ] Test rate limit headers present
- [ ] Test under limit allows request
- [ ] Test at limit returns 429
- [ ] Test key by IP
- [ ] Test key by user
- [ ] Test skip function bypasses
- [ ] Test burst allowance
- [ ] Test window reset
- [ ] Benchmark rate limit check (target: < 2ms)

---

## Phase 9: Maintenance Mode Middleware (`maintenance` package)

### 9.1 Redis Flag Interface
- [ ] Define `FlagStore` interface:
  - `IsEnabled(ctx context.Context) (bool, error)`
  - `GetMessage(ctx context.Context) (string, error)`
  - `GetEndTime(ctx context.Context) (*time.Time, error)`

### 9.2 Maintenance Middleware
- [ ] Implement `Middleware(store FlagStore, cfg Config) func(http.Handler) http.Handler`
- [ ] Check Redis flag for maintenance status
- [ ] If maintenance disabled: continue
- [ ] If bypass IP: continue
- [ ] If bypass path (e.g., /health): continue
- [ ] Otherwise: return 503 + SERVICE_UNAVAILABLE

### 9.3 Configuration
- [ ] Implement `Config` struct:
  - `BypassIPs []string` - IPs that bypass maintenance
  - `BypassPaths []string` - Paths that bypass (always include /health)
  - `DefaultMessage string` - Message when none in Redis
- [ ] Functional options

### 9.4 Response Format
- [ ] Include in response:
  - Error code: SERVICE_UNAVAILABLE
  - Message: Custom or default
  - Expected end time (if available)

### 9.5 Tests
- [ ] Test maintenance off → continues
- [ ] Test maintenance on → 503
- [ ] Test bypass IP → continues
- [ ] Test bypass path → continues
- [ ] Test custom message returned
- [ ] Test expected end time included

---

## Phase 10: Chain & Integration

### 10.1 Middleware Chain Helper
- [ ] Implement `Chain(middlewares ...func(http.Handler) http.Handler) func(http.Handler) http.Handler`
- [ ] Apply middlewares in order (first middleware wraps outermost)
- [ ] Support empty chain (no-op)

### 10.2 Standard Chain
- [ ] Implement `StandardChain(cfg StandardConfig) func(http.Handler) http.Handler`
- [ ] Pre-configured chain with recommended order:
  1. recovery
  2. request_id
  3. logging
  4. timeout
  5. cors
  6. maintenance
  7. ratelimit
  8. auth (optional based on config)
- [ ] Allow selective enable/disable of each middleware

### 10.3 Middleware Groups
- [ ] Implement `Group` type for reusable middleware sets
- [ ] Implement `NewGroup(middlewares ...func(http.Handler) http.Handler) *Group`
- [ ] Implement `(*Group).Use(middleware func(http.Handler) http.Handler)`
- [ ] Implement `(*Group).Handler(final http.Handler) http.Handler`

### 10.4 Integration Testing
- [ ] Full request flow through standard chain
- [ ] Auth → RBAC chain integration
- [ ] Rate limit with Redis mock
- [ ] Maintenance mode toggle

### 10.5 Final Validation
- [ ] Run full test suite: `go test -race -cover ./...`
- [ ] Verify > 85% coverage target
- [ ] Run linting: `golangci-lint run ./...`
- [ ] Run security analysis: `gosec ./...` (via golangci-lint)
- [ ] Fix all issues

---

## Success Criteria

| Metric | Target |
|--------|--------|
| Test coverage | > 85% |
| Auth middleware latency | < 5ms |
| Rate limit check latency | < 2ms |
| Panic recovery rate | 100% |
| Zero critical linting issues | Required |
| All gosec issues resolved | Required |
| All exports documented | Required |
