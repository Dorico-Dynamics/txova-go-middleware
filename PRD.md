# txova-go-middleware

## Overview
HTTP middleware library providing authentication, authorization, rate limiting, logging, and request processing middleware for Chi router.

**Module:** `github.com/txova/txova-go-middleware`

---

## Middleware Components

### `auth` - Authentication Middleware

**Function:** Validate JWT token and inject claims into context

**Behavior:**
| Scenario | Response |
|----------|----------|
| No Authorization header | 401 + TOKEN_REQUIRED |
| Invalid format | 401 + TOKEN_INVALID |
| Expired token | 401 + TOKEN_EXPIRED |
| Invalid signature | 401 + TOKEN_INVALID |
| Valid token | Inject claims, continue |

**Claims Injected:**
| Field | Context Key | Description |
|-------|-------------|-------------|
| user_id | ctx.user_id | Authenticated user |
| user_type | ctx.user_type | rider, driver, admin |
| roles | ctx.roles | Permission roles |

**Requirements:**
- Extract token from "Authorization: Bearer {token}"
- Validate signature using configured public key
- Validate expiration time
- Skip auth for paths in exclude list
- Log auth failures with IP address

---

### `optional_auth` - Optional Authentication

**Function:** Extract claims if token present, but don't require it

**Use Case:** Endpoints that behave differently for authenticated users

**Behavior:**
| Scenario | Response |
|----------|----------|
| No token | Continue (no claims) |
| Invalid token | Continue (no claims) |
| Valid token | Inject claims, continue |

---

### `rbac` - Role-Based Access Control

**Functions:**

| Middleware | Description |
|------------|-------------|
| RequireRole(roles...) | User must have one of specified roles |
| RequirePermission(perms...) | User must have all specified permissions |
| RequireOwner(paramName) | User ID must match URL param |
| RequireUserType(types...) | User type must be one of specified |

**Responses:**
| Scenario | Response |
|----------|----------|
| Missing required role | 403 + FORBIDDEN |
| Missing permission | 403 + FORBIDDEN |
| Not resource owner | 403 + FORBIDDEN |
| Wrong user type | 403 + FORBIDDEN |

**Requirements:**
- Check claims from context (requires auth middleware first)
- Log all access denials
- Support composite checks (role OR owner)

---

### `ratelimit` - Rate Limiting Middleware

**Configuration:**
| Setting | Description |
|---------|-------------|
| limit | Requests per window |
| window | Time window duration |
| key_func | Function to extract rate limit key |
| skip_func | Function to bypass rate limiting |

**Key Strategies:**
| Strategy | Description |
|----------|-------------|
| ByIP | Rate limit by client IP |
| ByUser | Rate limit by user ID |
| ByEndpoint | Rate limit by endpoint path |
| Custom | Custom key function |

**Response Headers:**
| Header | Description |
|--------|-------------|
| X-RateLimit-Limit | Max requests |
| X-RateLimit-Remaining | Remaining requests |
| X-RateLimit-Reset | Reset timestamp |

**Behavior:**
| Scenario | Response |
|----------|----------|
| Under limit | Add headers, continue |
| At limit | 429 + RATE_LIMITED |

**Requirements:**
- Use Redis for distributed rate limiting
- Support burst allowance
- Configurable response (reject or delay)

---

### `logging` - Request Logging Middleware

**Log Fields:**
| Field | Description |
|-------|-------------|
| method | HTTP method |
| path | Request path |
| status | Response status code |
| duration_ms | Request duration |
| request_id | Correlation ID |
| user_id | Authenticated user (if any) |
| ip | Client IP address |
| user_agent | Client user agent |
| bytes_in | Request body size |
| bytes_out | Response body size |

**Log Levels:**
| Status Range | Level |
|--------------|-------|
| 2xx | INFO |
| 4xx | WARN |
| 5xx | ERROR |

**Requirements:**
- Log at request completion
- Exclude health check paths
- Mask sensitive query params (token, key)
- Include stack trace for 5xx

---

### `request_id` - Request ID Middleware

**Behavior:**
1. Check for X-Request-ID header
2. If present, use existing ID
3. If absent, generate new UUID
4. Inject into context
5. Add to response headers

**Requirements:**
- Propagate existing IDs for distributed tracing
- Use UUID v4 for new IDs
- Always add to response for client reference

---

### `timeout` - Request Timeout Middleware

**Configuration:**
| Setting | Default | Description |
|---------|---------|-------------|
| timeout | 30s | Request timeout |
| skip_paths | [] | Paths to skip |

**Behavior:**
| Scenario | Response |
|----------|----------|
| Request completes | Normal response |
| Timeout exceeded | 503 + REQUEST_TIMEOUT |

**Requirements:**
- Cancel context on timeout
- Allow handler to detect cancellation
- Skip for long-running endpoints (file upload)

---

### `recovery` - Panic Recovery Middleware

**Behavior:**
1. Wrap handler in defer/recover
2. On panic, log stack trace
3. Return 500 + INTERNAL_ERROR

**Log Fields:**
| Field | Description |
|-------|-------------|
| panic | Panic value |
| stack | Stack trace |
| request_id | For correlation |

**Requirements:**
- Never expose panic details to client
- Always log full stack trace
- Include request context in log

---

### `cors` - CORS Middleware

**Configuration:**
| Setting | Default | Description |
|---------|---------|-------------|
| allowed_origins | [] | Allowed origins |
| allowed_methods | GET,POST,PUT,DELETE | Allowed methods |
| allowed_headers | Content-Type,Authorization | Allowed headers |
| exposed_headers | X-Request-ID | Headers to expose |
| max_age | 86400 | Preflight cache |
| allow_credentials | true | Allow cookies |

**Requirements:**
- Handle preflight OPTIONS requests
- Validate origin against allowlist
- Support wildcard in development only

---

### `maintenance` - Maintenance Mode Middleware

**Configuration:**
| Setting | Description |
|---------|-------------|
| enabled | Enable/disable maintenance |
| message | Custom message |
| bypass_ips | IPs that bypass |
| bypass_paths | Paths that bypass |

**Behavior:**
| Scenario | Response |
|----------|----------|
| Maintenance off | Continue |
| Bypass IP | Continue |
| Bypass path | Continue |
| Otherwise | 503 + SERVICE_UNAVAILABLE |

**Requirements:**
- Check Redis flag for dynamic toggle
- Always allow health check paths
- Include expected end time in response

---

## Middleware Chain

**Recommended Order:**
1. recovery (catch panics)
2. request_id (inject correlation)
3. logging (log all requests)
4. timeout (enforce limits)
5. cors (handle preflight)
6. maintenance (block if needed)
7. ratelimit (throttle abuse)
8. auth (verify identity)
9. rbac (check permissions)

**Requirements:**
- Provide Chain() helper for composing
- Allow per-route middleware overrides
- Support middleware groups

---

## Dependencies

**Internal:**
- `txova-go-core`
- `txova-go-security`

**External:**
- `github.com/go-chi/chi/v5` — Router
- `github.com/go-chi/cors` — CORS handling

---

## Success Metrics
| Metric | Target |
|--------|--------|
| Test coverage | > 85% |
| Auth middleware latency | < 5ms |
| Rate limit check latency | < 2ms |
| Panic recovery rate | 100% |
