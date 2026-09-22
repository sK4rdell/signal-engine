# API Conventions

## Purpose

This document defines the HTTP API conventions for the backend template.

The API layer should remove repetitive transport code without hiding request flow.

The default goal is:

> feature handlers should focus on application behavior, while common HTTP concerns are handled centrally.

---

# 1. Base paths

Application APIs live under:

```text
/v1
```

Examples:

```text
POST /v1/auth/signup
POST /v1/auth/login
GET  /v1/opportunities
POST /v1/opportunities
```

Health endpoints may live outside API versioning:

```text
GET /healthz
GET /readyz
GET /metricsz   in-process counters and pool statistics as JSON
```

---

# 2. Router

Use:

```text
github.com/go-chi/chi/v5
```

Use standard `net/http` underneath.

Do not introduce another HTTP framework unless explicitly approved.

The router should remain thin.

Its responsibilities are primarily:

* route registration
* middleware composition
* path parameters
* route grouping

The router should not contain business logic.

---

# 3. Typed handler abstraction

Normal API endpoints should use the shared typed handler wrapper.

Conceptually:

```go
type HandlerFunc[Req any, Res any] func(
    context.Context,
    Req,
) (Res, error)
```

Example route registration:

```go
router.Post(
    "/v1/opportunities",
    api.Handle(
        http.StatusCreated,
        CreateOpportunity(service),
    ),
)
```

Example handler:

```go
func CreateOpportunity(
    service *Service,
) api.HandlerFunc[CreateOpportunityRequest, CreateOpportunityResponse] {
    return func(
        ctx context.Context,
        req CreateOpportunityRequest,
    ) (CreateOpportunityResponse, error) {
        opportunity, err := service.Create(ctx, req.Name)
        if err != nil {
            return CreateOpportunityResponse{}, err
        }

        return CreateOpportunityResponse{
            ID: opportunity.ID,
        }, nil
    }
}
```

The handler should normally not interact directly with:

* `http.ResponseWriter`
* raw request bodies
* URL parsing
* JSON decoding
* validation internals
* error serialization

---

# 4. Handler wrapper responsibilities

The shared wrapper owns the standard HTTP lifecycle:

```text
HTTP request
    ↓
request metadata
    ↓
bind path params
    ↓
bind query params
    ↓
decode JSON body
    ↓
structural validation
    ↓
invoke feature handler
    ↓
map application error
    ↓
serialize response
```

The wrapper should also integrate naturally with:

* request IDs
* logging context
* status codes
* content type
* panic recovery at the HTTP boundary

The wrapper must remain small and understandable.

Do not turn it into a custom web framework.

Implementation notes for `api.Handle(status, fn, opts...)`:

* The request type must be a struct. Fields tagged `path:"..."` or
  `query:"..."` must also carry `json:"-"`; registration panics otherwise,
  so mistakes surface at startup rather than at request time.
* Any other exported field is a JSON body field. A request type with body
  fields **requires** a body; a type without body fields ignores the body;
  embedding `api.OptionalBody` makes the body optional.
* A non-empty body must be sent with `Content-Type: application/json`
  (a charset parameter is fine); anything else answers
  `415 unsupported_media_type`.
* Bodies larger than `HTTP_MAX_BODY_BYTES` answer `413 request_too_large`.
  `api.WithBodyLimit(n)` raises the limit for one endpoint. Handlers that
  bypass the wrapper call `api.LimitBody` themselves.
* Response types that implement `api.CookieResponse` have their cookies set
  before the body is written; status `204` writes no body (use
  `api.NoContent` as the response type).
* Handlers read the authenticated identity with `api.MustPrincipal(ctx)` and
  the resolved account with `account.MustFromContext(ctx)`.

---

# 5. Request types

Endpoints should define typed request structs.

Inputs may come from:

* path parameters
* query parameters
* JSON body

Example:

```go
type UpdateOpportunityRequest struct {
    OpportunityID uuid.UUID `path:"opportunityID" json:"-" validate:"required"`

    DryRun bool `query:"dry_run" json:"-"`

    Name string `json:"name" validate:"required,min=2,max=120"`
}
```

Request types should make the endpoint contract obvious from Go code.

Do not pass `*http.Request` deep into application logic.

---

# 6. Path binding

Path parameters should bind into typed request fields.

Example route:

```text
PATCH /v1/opportunities/{opportunityID}
```

Request:

```go
type UpdateOpportunityRequest struct {
    OpportunityID uuid.UUID `path:"opportunityID" json:"-" validate:"required"`
}
```

Binding failures should result in a stable client error.

Example malformed UUID:

```text
PATCH /v1/opportunities/not-a-uuid
```

should fail before business logic executes: `400 invalid_request` with
`fields: {"opportunityID": "must be a valid UUID"}`.

---

# 7. Query binding

Query parameters should bind into typed request fields.

Example:

```text
GET /v1/opportunities?limit=50&status=new
```

Request:

```go
type ListOpportunitiesRequest struct {
    Limit  int    `query:"limit" validate:"min=1,max=100"`
    Status string `query:"status" validate:"omitempty,oneof=new contacted won lost"`
}
```

The binder should support common scalar types:

* string
* bool
* signed integers
* unsigned integers where useful
* floats where useful
* UUID
* time values where explicitly supported
* slices for repeated query values where useful

Invalid values must fail explicitly: `400 invalid_request` with a per-field
message. A scalar parameter sent twice is also rejected. Absent parameters
leave the field at its zero value (or nil for pointer fields), which is how
"optional with default" is expressed together with `validate:"omitempty,..."`.

Do not silently coerce malformed input to zero values.

---

# 8. JSON body decoding

JSON decoding must be strict.

Reject:

* malformed JSON
* unknown fields
* multiple top-level JSON documents
* oversized request bodies

Use:

```go
decoder.DisallowUnknownFields()
```

or equivalent behavior.

Example typo:

```json
{
  "emial": "foo@example.com"
}
```

must fail.

It must not silently become:

```go
Email == ""
```

---

# 9. Empty body semantics

The API layer should distinguish between:

* endpoint expects no body
* body is optional
* body is required

Do not silently treat an absent required body as an empty request struct.

If a required JSON body is missing, return a stable malformed-request error
(`400 invalid_request`, "A JSON request body is required"). The body must be
a single JSON object: arrays, scalars, `null` and trailing content are
rejected with `400 invalid_request`; an unknown field is reported by name
in `fields`.

---

# 10. Request size limits

Set a reasonable default maximum request body size.

The default API body limit should be conservative.

Endpoints that legitimately accept larger payloads should opt into a larger explicit limit.

Do not leave HTTP bodies effectively unlimited.

---

# 11. Structural validation

Use:

```text
github.com/go-playground/validator/v10
```

for structural/mechanical validation.

Examples:

* required
* email syntax
* UUID syntax
* min/max length
* numeric bounds
* enum values
* simple field relationships

Example:

```go
type SignupRequest struct {
    Email    string `json:"email" validate:"required,email,max=254"`
    Password string `json:"password" validate:"required,min=12,max=1024"`
}
```

The handler wrapper should automatically invoke validation after successful binding.

---

# 12. Validation boundary

Structural validation belongs at the transport boundary.

Examples:

```text
email syntax
required name
limit between 1 and 100
valid UUID
allowed enum
```

Business validation belongs in application/domain logic.

Examples:

```text
email already registered
invalid state transition
account cannot modify resource
resource belongs to another tenant
```

Do not perform database-backed checks inside validator functions.

---

# 13. Validation errors

Validation failures should use a stable API representation.

Example:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "The request contains invalid fields",
    "fields": {
      "email": "must be a valid email address",
      "password": "must contain at least 12 characters"
    }
  }
}
```

Do not expose raw validator library internals.

The mapping from validator errors to API field errors should be centralized.

---

# 14. Error response format

Normal API errors use:

```json
{
  "error": {
    "code": "opportunity_not_found",
    "message": "Opportunity not found"
  }
}
```

Validation errors may additionally include:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "The request contains invalid fields",
    "fields": {
      "name": "is required"
    }
  }
}
```

Never return:

* stack traces
* SQL fragments
* raw PostgreSQL errors
* wrapped internal error chains
* dependency internals
* secrets

---

# 15. Stable public error codes

The `code` field is machine-readable and part of the API contract.

General codes include:

```text
invalid_request          400  malformed body, parameter or cursor
validation_failed        422  structural validation
unauthorized             401  missing or invalid session
forbidden                403
not_found                404
conflict                 409
rate_limited             429
request_too_large        413
unsupported_media_type   415
origin_not_allowed       403  CSRF guard
method_not_allowed       405
internal_error           500
```

Feature-specific errors are encouraged where useful:

```text
email_already_registered
opportunity_not_found
invalid_state_transition
account_membership_required
```

Clients should rely on `code`, not parse human-readable messages.

Changing an established error code is an API contract change.

---

# 16. HTTP status conventions

Use HTTP status codes consistently.

Recommended baseline:

```text
200 OK
201 Created
204 No Content

400 Bad Request
401 Unauthorized
403 Forbidden
404 Not Found
409 Conflict
422 Unprocessable Entity
429 Too Many Requests

500 Internal Server Error
```

Suggested interpretation:

## 400

Malformed request.

Examples:

* malformed JSON
* invalid query syntax
* malformed path parameter
* unknown JSON field

## 401

Authentication missing or invalid.

## 403

User is authenticated but is not authorized for the action.

## 404

Resource does not exist, or tenant-safe behavior intentionally hides its existence.

## 409

Request conflicts with current state.

Examples:

* duplicate resource
* state conflict
* uniqueness conflict

## 422

Request structure is syntactically valid but field validation fails.

## 429

Rate limit exceeded.

## 500

Unexpected internal failure.

---

# 17. Internal failures

Unexpected errors should return a generic response.

Example:

```json
{
  "error": {
    "code": "internal_error",
    "message": "An unexpected error occurred"
  }
}
```

The real underlying error should remain available internally for logging.

Do not expose:

```text
pq: duplicate key value violates unique constraint...
```

or similar details.

---

# 18. Successful responses

Do not require a universal:

```json
{
  "data": ...
}
```

wrapper.

Prefer direct response objects.

Example:

```json
{
  "id": "0199...",
  "name": "Example"
}
```

Collection endpoints may return:

```json
{
  "items": [],
  "next_cursor": null
}
```

Use response wrappers only where they provide real value.

---

# 19. JSON naming

Use `snake_case` for JSON fields.

Example:

```json
{
  "account_id": "...",
  "created_at": "...",
  "email_verified_at": "..."
}
```

Do not mix:

```text
accountId
account_id
AccountID
```

across APIs.

---

# 20. Time representation

Expose timestamps as RFC3339.

Example:

```json
{
  "created_at": "2026-09-22T06:15:00Z"
}
```

Use UTC internally.

Do not expose ambiguous local timestamps.

---

# 21. Entity IDs

Entity identifiers should normally be UUIDs represented as JSON strings.

Prefer UUIDv7 for application-generated IDs.

Example:

```json
{
  "id": "0199..."
}
```

Security tokens are not entity IDs and should use independent cryptographically random values.

---

# 22. Pagination

For collections that may grow, prefer cursor-based pagination.

Example request:

```text
GET /v1/opportunities?limit=50&cursor=...
```

Example response:

```json
{
  "items": [],
  "next_cursor": "..."
}
```

Define:

* sensible default page size
* maximum page size

Never allow unbounded list endpoints by default.

---

# 23. Cursor behavior

Cursors should be opaque to clients.

Clients should not need to construct or parse them.

Ordering must be deterministic.

Example:

```text
ORDER BY created_at DESC, id DESC
```

The cursor should contain enough information to resume from the correct position.

Invalid cursors should return a stable client error.

---

# 24. Filtering

Expose supported filters explicitly.

Example:

```text
GET /v1/opportunities?status=new
```

Use typed validation for filter values.

Unsupported values should generally be rejected rather than silently ignored.

---

# 25. Sorting

Expose only known supported sorting options.

Example:

```text
?sort=created_at&order=desc
```

Never copy arbitrary client-provided sort fields directly into SQL.

Map public sort values to explicit internal columns.

---

# 26. Authentication

Browser/API authentication uses secure opaque server-side sessions.

Protected routes should use authentication middleware.

Handlers should receive authenticated identity from context.

Do not parse cookies manually inside feature handlers.

---

# 27. Principal

The authentication layer should expose a small request principal.

Conceptually:

```go
type Principal struct {
    UserID uuid.UUID
}
```

Only include information that is broadly useful and safe to propagate.

Do not place large mutable user models in request context.

---

# 28. Account authorization

Authentication answers:

> Who are you?

Authorization answers:

> May you access this account/resource?

Account-scoped operations must verify membership/ownership.

Do not trust:

```text
account_id
```

merely because it was sent by the client.

---

# 29. Tenant-safe resource behavior

For account-scoped resources, decide consistently whether unauthorized cross-account access produces:

```text
403 Forbidden
```

or:

```text
404 Not Found
```

A tenant-safe `404` is often appropriate when revealing resource existence would leak information.

Whichever convention is chosen should be used consistently.

---

# 30. Request context

Request context may contain safe infrastructure metadata such as:

```text
request_id
principal
account context where resolved
logger
```

Do not use context as a generic bag for arbitrary application state.

---

# 31. Request IDs

Every request should have a request ID.

The request ID should:

* be available in request-scoped logging
* optionally be returned in a response header
* be generated if absent

If accepting an externally supplied request ID:

* validate length
* validate allowed characters
* do not blindly trust arbitrary huge values

---

# 32. Request logging

Request logs should normally include:

```text
request_id
method
path
status
duration
user_id
account_id
```

where available.

Do not log:

* authorization headers
* raw cookies
* passwords
* reset tokens
* verification tokens
* complete sensitive request bodies

---

# 33. Panic recovery

Recover panics at the HTTP boundary.

A panic should:

1. produce a generic `500` response
2. be logged with enough diagnostic context
3. not crash the entire server process

Panic recovery is not a substitute for ordinary error handling.

---

# 34. Middleware

Keep middleware small and explicit.

Expected middleware includes:

* request ID
* panic recovery
* structured request logging
* trusted client IP handling
* security headers
* CORS
* authentication where required
* rate limiting where required

Middleware ordering (`api.NewRouter`, outermost first):

```text
1. RequestID        assigns/echoes X-Request-ID, opens the logging scope
2. RealIP           resolves the client IP, honouring X-Forwarded-For only
                    from HTTP_TRUSTED_PROXIES
3. RequestLogger    one structured line per request, metrics
4. Recover          panic -> 500 internal_error, logged with stack
5. SecurityHeaders  nosniff, DENY framing, no-referrer, no-store, CSP;
                    HSTS only with HTTP_HSTS=true
6. CORS             preflight and headers for HTTP_ALLOWED_ORIGINS,
                    credentials allowed, origin echoed exactly
7. TrustedOrigin    CSRF guard for unsafe methods
8. BodyLimit        default body limit used by api.Handle
```

Authentication (`auth.RequireSession`), membership
(`account.RequireMembership`) and rate limiting (`api.RateLimit`) are
applied per route group by the feature packages.

---

# 35. CORS

CORS configuration is environment/config driven.

Production frontend origins should be explicitly configured.

Do not use:

```text
Access-Control-Allow-Origin: *
```

with credentialed browser requests.

---

# 36. CSRF

Because authentication uses cookies, unsafe browser requests require CSRF consideration.

Implementation (`api.TrustedOrigin`, applied to every route):

* Unsafe methods are everything except `GET`, `HEAD` and `OPTIONS`.
* An unsafe request whose `Origin` header is not in `HTTP_ALLOWED_ORIGINS`
  answers `403 origin_not_allowed`. Comparison is case-insensitive on the
  full origin (`scheme://host[:port]`).
* Without `Origin`, the `Referer` origin is checked the same way when
  present.
* Requests with neither header (curl, server-to-server) pass: browsers
  always attach `Origin` to cross-site unsafe requests and to fetch/XHR.
* The session cookie is `SameSite=Lax`, an independent second layer.

Threat model: a malicious site making the victim's browser send a
state-changing request with the session cookie attached. Not covered:
compromised allowed origins, or clients that deliberately strip headers
(those are not browsers acting on a victim's behalf).

Do not assume CORS alone prevents CSRF.

---

# 37. Cookies

Production auth cookies should normally use:

```text
HttpOnly
Secure
SameSite=Lax
Path=/
```

Cookie configuration may differ for unusual deployment topology, but deviations should be deliberate.

Never expose raw session tokens to frontend JavaScript unless architecture explicitly requires it.

---

# 38. Rate limiting

Sensitive endpoints require rate limiting.

Implemented with `api.RateLimit` and an in-memory fixed-window limiter per
client IP (`AUTH_RATE_LIMIT_REQUESTS` per `AUTH_RATE_LIMIT_WINDOW`, one
limiter per endpoint) on:

```text
signup
login
verify email
resend verification
forgot password
reset password
```

Blocked requests answer `429 rate_limited` with `Retry-After`. Resend
verification additionally enforces a per-user cooldown
(`AUTH_RESEND_COOLDOWN`).

The limiter state is per process: with several replicas the effective
limit is multiplied by the replica count. Acceptable for abuse protection;
move the state to PostgreSQL if exact global limits matter.

---

# 39. Health endpoints

## `/healthz`

Means:

> the process is alive

It should not depend on PostgreSQL or other external systems.

## `/readyz`

Means:

> the process is ready to serve application traffic

It should at minimum confirm PostgreSQL connectivity.

Keep these endpoints lightweight.

---

# 40. Resource-oriented routes

Prefer predictable resource-oriented APIs.

Examples:

```text
POST   /v1/opportunities
GET    /v1/opportunities
GET    /v1/opportunities/{id}
PATCH  /v1/opportunities/{id}
DELETE /v1/opportunities/{id}
```

Do not force everything into CRUD when a named action is clearer.

Example:

```text
POST /v1/auth/logout
POST /v1/opportunities/{id}/archive
```

Clarity is more important than REST purity.

---

# 41. PATCH semantics

PATCH request types must distinguish between:

```text
field absent
```

and:

```text
field explicitly set to zero/null/empty value
```

where semantics require the distinction.

Example:

```go
type UpdateUserRequest struct {
    Name *string `json:"name" validate:"omitempty,min=2,max=100"`
}
```

Do not accidentally interpret omitted values as update-to-zero.

---

# 42. DELETE semantics

For normal successful deletion, prefer:

```text
204 No Content
```

unless a useful deleted-resource representation is deliberately returned.

Deletion authorization and account scoping must be enforced.

---

# 43. Idempotency

Do not add idempotency machinery to every endpoint.

Use idempotency where retries can cause costly or incorrect duplication.

Examples:

* payment creation
* external provisioning
* certain imports
* retried webhooks

A future endpoint may support:

```text
Idempotency-Key
```

when needed.

---

# 44. Webhooks

Webhook endpoints may need to bypass the normal JSON wrapper when signature verification requires the raw request body.

Webhook handlers should:

1. read bounded raw body
2. verify signature
3. reject invalid signatures
4. decode payload
5. enforce idempotency/replay protection
6. enqueue slow work where appropriate

Document why the standard wrapper is bypassed.

---

# 45. File uploads

Do not force file uploads through JSON.

Use appropriate multipart or direct-object-storage patterns when a product requires them.

Apply explicit size limits.

Do not add generic upload infrastructure to the base template before a real use case exists.

---

# 46. File downloads

Streaming/file-response endpoints may use normal `net/http` behavior directly.

They are legitimate exceptions to the typed JSON response wrapper.

Authorization and error behavior must still follow platform conventions.

---

# 47. Content types

Normal API requests and responses use:

```text
application/json
```

Return correct `Content-Type`.

Endpoints with specialized content types should set them explicitly.

---

# 48. Versioning

The base API starts with:

```text
/v1
```

Do not create `/v2` for minor compatible changes.

Use a new API version only for substantial incompatible API contracts.

---

# 49. Backwards compatibility

During rapid MVP development, backwards compatibility requirements may be relaxed before external clients depend on the API.

Once a contract is externally consumed:

* avoid silently changing field meaning
* avoid changing stable error codes
* avoid removing response fields without migration
* document breaking changes

Do not overengineer compatibility for prototypes that have no consumers.

---

# 50. Public vs internal fields

Do not serialize internal database models directly merely because fields currently align.

Prefer explicit API response types.

This prevents accidental exposure when persistence models evolve.

Example:

```go
type UserResponse struct {
    ID    uuid.UUID `json:"id"`
    Email string    `json:"email"`
}
```

Never accidentally expose:

```text
password_hash
session_hash
internal flags
```

---

# 51. API models vs domain models

Do not create unnecessary duplicate types everywhere.

Create separate API types where:

* public shape differs from internal model
* sensitive fields must be excluded
* PATCH presence semantics are needed
* transport-specific validation exists
* backwards compatibility matters

Otherwise reuse simple safe domain types where appropriate.

Use judgment.

---

# 52. API testing

Normal feature endpoints should be tested through the real router.

A feature integration test should ideally exercise:

```text
HTTP
 ↓
router
 ↓
middleware
 ↓
binding
 ↓
validation
 ↓
auth
 ↓
handler
 ↓
repository/service
 ↓
PostgreSQL
```

This ensures endpoint behavior is tested as clients actually experience it.

See:

```text
docs/testing.md
```

---

# 53. Handler wrapper testing

The typed wrapper is critical platform infrastructure.

It must have dedicated coverage for:

* valid JSON
* malformed JSON
* unknown fields
* multiple JSON documents
* body size limit
* path binding
* query binding
* validation
* application errors
* unexpected errors
* response encoding
* content type
* request IDs

Feature tests should not need to repeat all these cases unless feature-specific behavior differs.

---

# 54. API documentation

The template does not require a full OpenAPI generation framework initially.

Endpoint contracts should remain discoverable through:

* request/response Go types
* tests
* feature documentation where needed

If a product later requires public API documentation, OpenAPI can be added deliberately.

Do not add annotation-heavy tooling by default.

---

# 55. Success criterion

The API layer is successful when adding a normal endpoint mostly requires:

```text
define request type
define response type
write application behavior
register route
write tests
```

It should not require repeatedly writing:

```text
decode JSON
validate request
write headers
serialize errors
attach request IDs
map every error manually
```

The abstraction should remove boilerplate while keeping execution flow obvious.
