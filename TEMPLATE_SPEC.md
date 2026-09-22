# Go Backend Template — Implementation Specification

## 1. Purpose

Build a production-capable but deliberately small Go backend template optimized for rapidly validating new product ideas.

The intended workflow is:

```bash
create repo from template
cp .env.example .env
docker compose up -d
make migrate
make dev
```

After that, development should focus almost entirely on product-specific features.

The template must solve recurring backend concerns once:

* HTTP server and routing
* request binding and validation
* consistent responses
* error handling
* structured logging
* PostgreSQL access
* safe transaction handling
* migrations
* authentication
* authorization foundations
* accounts/workspaces
* signup/login/logout
* email verification
* password reset
* email delivery
* background jobs
* configuration
* health/readiness
* graceful shutdown
* security middleware
* integration testing
* Docker
* CI
* developer tooling

The template must remain understandable to a senior Go developer without learning a custom framework.

---

# 2. Core principles

## 2.1 Optimize for iteration speed

This repository exists to help validate product ideas quickly.

Prefer:

* explicit code
* small abstractions
* predictable conventions
* reusable infrastructure
* sensible defaults

Avoid:

* speculative abstractions
* generic frameworks
* excessive indirection
* premature scalability work
* unnecessary infrastructure

---

## 2.2 Package by feature

Application/domain code is organized by feature.

Preferred:

```text
internal/
  auth/
  account/
  company/
  opportunity/
```

Avoid architectural folders such as:

```text
handlers/
repositories/
services/
models/
```

containing unrelated features.

Infrastructure shared across features lives under:

```text
internal/platform/
```

---

## 2.3 HTTP handlers must not contain SQL

Handlers are transport adapters.

Their responsibilities are:

* receive validated typed input
* obtain authenticated principal/context
* call application/domain logic
* return typed output or error

SQL belongs in feature-local repository files.

---

## 2.4 Repositories contain persistence logic

Files should preferably follow conventions such as:

```text
foo.handler.go
foo.service.go
foo.repository.go
foo.model.go
```

Do not create `service.go` merely to satisfy the architecture.

This is valid:

```text
handler -> repository
```

when the operation has trivial business logic.

Use:

```text
handler -> service -> repository
```

when meaningful business logic exists.

---

## 2.5 Prefer feature-local code

Do not prematurely move code into shared packages.

A useful rule:

> Shared abstractions should generally exist because at least two real features need them.

Infrastructure concerns such as database, HTTP, logging, auth primitives, configuration and email are exceptions.

---

# 3. Reference code in `old/`

An existing Bolagsbild backend will initially be placed under:

```text
old/
```

It exists only as a reference.

Claude may inspect `old/` to find:

* useful SQL/query patterns
* existing implementations
* lessons from previous code
* domain-independent utilities worth adapting

However:

* `old/` is NOT the architectural source of truth.
* Never import packages from `old/`.
* Never create runtime dependencies on `old/`.
* Never mass-copy `old/`.
* Never preserve an implementation merely because it exists there.
* The new specification and `CLAUDE.md` take precedence.
* Reuse ideas or small implementations only when they fit the new architecture.
* Rewrite code when the new conventions require it.
* Do not modify `old/` unless explicitly instructed.

Create:

```text
old/README.md
```

explaining these rules.

---

# 4. Technology choices

Use the following defaults.

## Language

Go 1.26+.

Do not unnecessarily require a newer Go version unless a chosen dependency requires it.

## HTTP

Use:

```text
github.com/go-chi/chi/v5
```

Use standard `net/http` types underneath.

Do not introduce Gin, Echo, Fiber or another HTTP framework.

The custom typed handler wrapper described below should be the main opinionated HTTP abstraction.

## Database

Use:

```text
github.com/jackc/pgx/v5
github.com/jackc/pgx/v5/pgxpool
```

Use pgx directly.

Do not use:

* GORM
* Ent
* sqlx
* another ORM
* a generated repository framework

SQL should remain explicit.

## Migrations

Use:

```text
github.com/pressly/goose/v3
```

Prefer SQL migrations.

## Validation

Use:

```text
github.com/go-playground/validator/v10
```

for structural/mechanical validation.

## Logging

Use standard library:

```text
log/slog
```

Avoid introducing another logging framework unless there is a demonstrated requirement.

## Password hashing

Use Argon2id through:

```text
golang.org/x/crypto/argon2
```

Store all parameters required to verify and later upgrade password hashes.

## IDs

Prefer UUIDv7 for application-generated entity identifiers.

Use one established UUID library consistently.

---

# 5. Target repository structure

Initial structure:

```text
.
├── cmd/
│   ├── api/
│   │   └── main.go
│   ├── worker/
│   │   └── main.go
│   └── migrate/
│       └── main.go
│
├── internal/
│   ├── auth/
│   │   ├── auth.model.go
│   │   ├── auth.repository.go
│   │   ├── login.handler.go
│   │   ├── login.service.go
│   │   ├── logout.handler.go
│   │   ├── signup.handler.go
│   │   ├── signup.service.go
│   │   ├── verify_email.handler.go
│   │   ├── verify_email.service.go
│   │   ├── forgot_password.handler.go
│   │   ├── reset_password.handler.go
│   │   └── reset_password.service.go
│   │
│   ├── account/
│   │   ├── account.model.go
│   │   ├── account.repository.go
│   │   └── account.service.go
│   │
│   ├── platform/
│   │   ├── api/
│   │   ├── apperror/
│   │   ├── config/
│   │   ├── database/
│   │   ├── email/
│   │   ├── jobs/
│   │   ├── logging/
│   │   ├── server/
│   │   └── testutil/
│   │
│   └── ...
│
├── migrations/
├── docs/
│   ├── architecture.md
│   ├── api-conventions.md
│   ├── database.md
│   └── testing.md
│
├── old/
│   └── README.md
│
├── .github/
│   └── workflows/
│       └── ci.yml
│
├── .env.example
├── compose.yaml
├── Dockerfile
├── Makefile
├── CLAUDE.md
├── TEMPLATE_SPEC.md
├── go.mod
└── go.sum
```

Do not create empty architectural layers without a real purpose.

---

# 6. Configuration

Implement a typed configuration package.

Example:

```go
type Config struct {
    Environment string
    HTTP        HTTPConfig
    Database    DatabaseConfig
    Auth        AuthConfig
    Email       EmailConfig
    Jobs        JobsConfig
}
```

Requirements:

* configuration comes from environment variables
* required values are validated at startup
* invalid configuration prevents startup
* secrets must never be logged
* `.env.example` documents all available configuration
* sensible local-development defaults are allowed
* production must not silently fall back to insecure defaults

Configuration loading should be explicit and small.

Do not introduce Viper or a large configuration framework.

---

# 7. Database layer

## 7.1 Pool

Use `pgxpool.Pool`.

Configure:

* maximum connections
* minimum/idle connections where useful
* max connection lifetime
* max connection idle time
* health check interval

Validate database connectivity during startup.

---

## 7.2 DBTX abstraction

Repositories should normally accept a minimal interface implemented by both:

* `*pgxpool.Pool`
* `pgx.Tx`

Example concept:

```go
type DBTX interface {
    Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
    Query(context.Context, string, ...any) (pgx.Rows, error)
    QueryRow(context.Context, string, ...any) pgx.Row
}
```

This allows repositories to participate in transactions without alternate implementations.

---

# 8. Transaction helper

Implement a safe transaction helper.

Desired usage:

```go
err := database.InTx(ctx, pool, func(tx pgx.Tx) error {
    if err := repoA.DoSomething(ctx, tx, ...); err != nil {
        return err
    }

    if err := repoB.DoSomething(ctx, tx, ...); err != nil {
        return err
    }

    return nil
})
```

Requirements:

* transaction begins safely
* rollback happens automatically on all non-commit paths
* commit happens only after callback success
* original error is preserved
* connection is always returned to the pool
* context cancellation is respected
* callers should not manually acquire a pooled connection simply to start a transaction

Add tests covering:

* successful commit
* callback error
* commit failure where testable
* context cancellation
* no connection leak

Do not build a generalized transaction framework.

---

# 9. Migrations

Use goose SQL migrations.

Requirements:

```bash
make migrate
make migrate-down
make migration name=create_foo
make migration-status
```

Migration files live under:

```text
migrations/
```

Application startup must NOT automatically apply schema migrations in production.

Migration execution should have a dedicated command:

```text
cmd/migrate
```

---

# 10. HTTP server

Use `chi` + `net/http`.

Server configuration must include:

* address/port
* read header timeout
* read timeout where appropriate
* write timeout where appropriate
* idle timeout
* graceful shutdown timeout
* maximum request body size

Implement graceful shutdown:

```text
SIGTERM/SIGINT
  ->
stop accepting requests
  ->
allow active requests to finish within timeout
  ->
stop worker
  ->
close database pool
  ->
exit
```

---

# 11. Typed handler wrapper

This is a central part of the template.

The application should expose a small typed HTTP abstraction so product handlers contain almost no transport boilerplate.

Concept:

```go
type HandlerFunc[Req any, Res any] func(
    context.Context,
    Req,
) (Res, error)
```

Desired usage:

```go
router.Post(
    "/v1/foo",
    api.Handle(http.StatusCreated, CreateFoo(service)),
)
```

Actual handler:

```go
func CreateFoo(service *Service) api.HandlerFunc[CreateFooRequest, CreateFooResponse] {
    return func(
        ctx context.Context,
        req CreateFooRequest,
    ) (CreateFooResponse, error) {
        foo, err := service.Create(ctx, req.Name)
        if err != nil {
            return CreateFooResponse{}, err
        }

        return CreateFooResponse{
            ID: foo.ID,
        }, nil
    }
}
```

The wrapper should perform:

```text
HTTP request
  ->
request binding
  ->
structural validation
  ->
handler execution
  ->
error mapping
  ->
response serialization
```

Logging and request metadata should integrate naturally with this lifecycle.

Do not turn the wrapper into a large custom framework.

---

# 12. Request binding

Support typed binding from:

* JSON body
* path parameters
* query parameters

Example:

```go
type UpdateFooRequest struct {
    FooID uuid.UUID `path:"fooID" json:"-" validate:"required"`
    Force bool      `query:"force" json:"-"`
    Name  string    `json:"name" validate:"required,min=2,max=100"`
}
```

The generic binder should support common scalar types:

* string
* bool
* signed integers
* unsigned integers where useful
* float where useful
* UUID
* time.Time where useful
* slices for repeated query parameters where practical

Do not implement arbitrary reflection magic beyond what is needed.

Unknown or unsupported conversions should fail clearly.

---

# 13. JSON decoding

JSON requests must use strict decoding.

Requirements:

* reject malformed JSON
* reject unknown fields
* reject more than one JSON object
* enforce maximum body size
* empty body handling must be explicit
* distinguish malformed request from field validation failure

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

must fail rather than silently ignoring `emial`.

---

# 14. Input validation

Use validator/v10 for structural validation.

Example:

```go
type SignupRequest struct {
    Email    string `json:"email" validate:"required,email,max=254"`
    Password string `json:"password" validate:"required,min=12,max=1024"`
}
```

The handler wrapper should automatically validate bound request structs.

Structural validation belongs here:

* required
* email
* length
* numeric ranges
* UUID format
* allowed enum
* field relationships that do not require persistent state

Business validation does NOT belong here.

Examples:

```text
"email must be valid"
```

-> request validation

```text
"email is already registered"
```

-> application/domain logic

---

# 15. Validation error format

Use a stable machine-readable response.

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

Never expose validator library internals directly to clients.

Map validator errors centrally.

---

# 16. API error model

Create:

```text
internal/platform/apperror
```

Application errors should separate:

* public code
* public message
* HTTP status
* internal cause
* optional field errors
* optional operation/context

Conceptually:

```go
type Error struct {
    Code       string
    Message    string
    HTTPStatus int
    Cause      error
    Fields     map[string]string
}
```

Preserve wrapped errors using standard Go error semantics.

Support errors such as:

```text
invalid_request
validation_failed
unauthorized
forbidden
not_found
conflict
rate_limited
internal_error
```

Recommended mapping:

```text
400 malformed request
401 unauthenticated
403 forbidden
404 not found
409 conflict
422 structural validation
429 rate limited
500 unexpected internal failure
```

Product-specific stable error codes are allowed.

---

# 17. Error handling boundary

Errors should generally be logged once at a boundary.

For HTTP:

```text
handler -> central HTTP error handler -> logger
```

For jobs:

```text
job -> worker boundary -> logger
```

Avoid logging the same error at every layer.

Internal error details must never leak into HTTP responses.

Unexpected errors return a generic message.

The internal logger should retain the original cause.

---

# 18. Response encoding

Provide centralized helpers for:

* JSON responses
* errors
* no-content responses

Use:

```text
Content-Type: application/json
```

consistently.

Do not introduce a mandatory envelope such as:

```json
{"data": ...}
```

unless the product later has a reason for it.

---

# 19. API conventions

Base prefix:

```text
/v1
```

Dates/timestamps:

```text
RFC3339
UTC internally
```

Pagination:

prefer cursor-based pagination for potentially large collections.

Example:

```text
?limit=50&cursor=...
```

Enforce a maximum page size.

IDs:

prefer UUIDv7.

---

# 20. Middleware

Implement a small, explicit middleware stack.

Required:

* request ID
* panic recovery
* structured request logging
* real client IP handling
* security headers
* CORS
* request body limits
* authentication
* rate limiting for sensitive endpoints

Middleware order must be documented.

Do not add middleware packages merely because they exist.

---

# 21. Request context

Every request should have contextual logging fields including when known:

```text
request_id
method
path
user_id
account_id
```

Provide helpers such as:

```go
logger := logging.FromContext(ctx)
```

Feature code should be able to log:

```go
logger.Info(
    "opportunity created",
    "opportunity_id", opportunity.ID,
)
```

without manually re-attaching request metadata.

---

# 22. Logging

Use `log/slog`.

Local development:

* human-readable text is acceptable

Production:

* JSON structured logging

Include useful fields:

```text
timestamp
level
message
request_id
method
path
status
duration
user_id
account_id
operation
error
```

Configure source information for errors/debugging where useful.

Never log:

* passwords
* reset tokens
* session tokens
* verification tokens
* authorization headers
* raw cookies
* secrets
* complete sensitive request bodies

---

# 23. Health and readiness

Implement:

```text
GET /healthz
GET /readyz
```

`/healthz`:

* indicates process is alive
* must not depend on external systems

`/readyz`:

* confirms the application is able to serve traffic
* at minimum validates database connectivity

Keep these endpoints lightweight.

---

# 24. Authentication model

Use first-party authentication.

Do not use JWT for ordinary browser sessions.

Use opaque server-side sessions.

Core tables:

```text
users
accounts
account_memberships
sessions
email_verification_tokens
password_reset_tokens
```

---

# 25. User model

At minimum:

```text
id
email
email_normalized
password_hash
email_verified_at
created_at
updated_at
```

Requirements:

* emails normalized consistently
* normalized email must be unique
* original display email may be preserved
* password hashes never leave auth internals

---

# 26. Account/workspace model

Do not model application ownership as:

```text
resource.user_id
```

by default.

Use:

```text
users
accounts
account_memberships
```

A signup may initially create:

```text
user
personal/default account
owner membership
```

This avoids later migration from:

```text
one user = one tenant
```

to multi-user accounts.

Initial membership roles can remain small:

```text
owner
member
```

Do not build a complex RBAC framework.

---

# 27. Password storage

Use Argon2id.

Store encoded parameters with each password hash so cost parameters can evolve.

Provide:

```go
HashPassword(...)
VerifyPassword(...)
```

Implement reasonable password constraints.

Do not invent arbitrary complexity rules such as requiring uppercase + symbol + number.

Prefer length and allow password managers.

---

# 28. Sessions

Sessions must use cryptographically random opaque tokens.

Client receives token only in a secure cookie.

Store only a cryptographic hash of the token in PostgreSQL.

Cookie requirements in production:

```text
HttpOnly
Secure
SameSite=Lax
Path=/
```

Session record should support:

```text
id
user_id
token_hash
expires_at
created_at
last_seen_at
revoked_at or deletion
```

Never store plain session tokens.

Implement:

* current session logout
* logout all sessions
* expiration
* session lookup
* session revocation

Password reset should invalidate existing sessions.

---

# 29. Signup

Implement:

```text
POST /v1/auth/signup
```

Flow:

```text
validate input
  ->
normalize email
  ->
check/create user
  ->
hash password
  ->
create account
  ->
create owner membership
  ->
create email verification token
  ->
enqueue verification email
  ->
create session where policy allows
```

Operations that must succeed atomically should run in one transaction.

Do not send SMTP messages while holding a database transaction.

Queue them transactionally instead.

---

# 30. Email verification

Implement:

```text
POST /v1/auth/verify-email
POST /v1/auth/resend-verification
```

Verification tokens must:

* be cryptographically random
* be single-use
* expire
* be stored hashed
* never be logged

Resend must be rate limited.

---

# 31. Login

Implement:

```text
POST /v1/auth/login
```

Requirements:

* normalized email lookup
* constant/safe password verification behavior
* session creation
* rate limiting
* avoid useful account enumeration

Authentication failures should normally return the same public error regardless of whether the email exists.

---

# 32. Logout

Implement:

```text
POST /v1/auth/logout
POST /v1/auth/logout-all
```

Current logout invalidates only current session.

Logout-all invalidates all sessions for the user.

---

# 33. Forgot/reset password

Implement:

```text
POST /v1/auth/forgot-password
POST /v1/auth/reset-password
```

Forgot-password response must not reveal whether the email exists.

Reset tokens:

* cryptographically random
* stored hashed
* expire
* single-use

Successful reset:

* updates password
* consumes reset token
* invalidates existing sessions

---

# 34. CSRF protection

Because authentication uses cookies, state-changing browser requests require CSRF consideration.

For JSON APIs consumed by an owned frontend:

* SameSite cookie protection
* validate allowed `Origin` for unsafe methods
* reject untrusted origins

Document the threat model.

Do not assume CORS itself is CSRF protection.

---

# 35. Rate limiting

At minimum rate-limit:

* login
* signup
* resend verification
* forgot password
* reset password

An in-memory limiter is acceptable for the initial template if:

* implementation is explicit
* limitation in multi-instance deployments is documented

Do not add Redis solely for rate limiting.

---

# 36. Email abstraction

Create:

```text
internal/platform/email
```

Core interface:

```go
type Sender interface {
    Send(context.Context, Message) error
}
```

Support:

* subject
* recipients
* text body
* HTML body

Templates should live in the repository and be embedded into the binary where practical.

Initial templates:

```text
verify email
reset password
welcome
```

Do not couple auth directly to a third-party email vendor.

---

# 37. Local email development

Use Mailpit in Docker Compose.

Developers must be able to:

* signup locally
* inspect verification email in Mailpit
* click/use verification token
* request password reset
* inspect reset email

Document Mailpit's local UI URL.

---

# 38. Background job system

Implement a deliberately small PostgreSQL-backed background job queue.

Purpose:

* send emails
* run small asynchronous product operations
* avoid blocking HTTP requests
* allow retries

Use:

```text
cmd/worker
```

A possible table shape:

```text
jobs
  id
  type
  payload
  status
  attempts
  max_attempts
  run_at
  locked_at
  locked_by
  last_error
  created_at
  completed_at
```

Use PostgreSQL locking such as:

```text
FOR UPDATE SKIP LOCKED
```

for safe concurrent workers.

Requirements:

* retry with bounded backoff
* max attempts
* graceful worker shutdown
* structured logs
* unknown job types fail clearly
* jobs can be enqueued inside application transactions

Keep this simple.

Do not build Temporal.

---

# 39. Transactional asynchronous work

Where an operation both modifies state and schedules work:

```text
create user
+
enqueue verification email
```

both database changes should happen in the same transaction.

The worker performs the actual external email call after commit.

This avoids:

```text
database committed but email enqueue lost
```

and:

```text
email sent but database rollback
```

---

# 40. Security headers

Set suitable API security headers centrally.

At minimum consider:

```text
X-Content-Type-Options: nosniff
```

and other relevant headers based on deployment.

HSTS should only be emitted when HTTPS deployment semantics are understood.

Do not cargo-cult browser headers that are irrelevant to a JSON API.

---

# 41. CORS

CORS must be configuration-driven.

Production should explicitly list allowed frontend origins.

Do not use unrestricted:

```text
Access-Control-Allow-Origin: *
```

with credentials.

---

# 42. Trusted proxies / client IP

Do not blindly trust forwarded IP headers.

Trusted proxy behavior must be configuration-driven.

The request logger and rate limiter must share the same client-IP resolution logic.

---

# 43. Observability

Structured logging is mandatory.

Additionally prepare lightweight hooks for:

* request count
* request duration
* HTTP error count
* DB pool statistics
* job success/failure

Do not require Grafana/Prometheus/Tempo/Loki for local development.

OpenTelemetry integration may be added if it remains small and optional.

The template should not boot an observability stack by default.

---

# 44. Docker Compose

Provide:

```text
compose.yaml
```

with at least:

```text
postgres
mailpit
```

Use a modern supported PostgreSQL version.

Configure persistent volume for local database data.

Add health checks.

Do not run the Go application in Compose by default unless it improves the developer loop.

Local Go execution against containerized dependencies is preferred.

---

# 45. Dockerfile

Provide a production multi-stage Dockerfile.

Requirements:

* deterministic Go build
* non-root final user
* minimal runtime image
* copy only required runtime artifacts
* expose no secrets
* support build metadata when practical

The same image should be suitable for common container hosting.

---

# 46. Development commands

Provide a Makefile.

At minimum:

```bash
make dev
make build
make test
make test-race
make lint
make fmt
make vet

make migrate
make migrate-down
make migration name=...
make migration-status

make compose-up
make compose-down
make reset-db
```

Commands should be discoverable:

```bash
make help
```

---

# 47. Testing philosophy

Prefer real behavior over mocks.

Three levels:

## Unit tests

For pure application/domain logic.

No database required.

## Repository integration tests

Run against real PostgreSQL.

Test actual SQL.

## HTTP integration tests

Run real:

```text
router
handler wrapper
auth middleware
service
repository
PostgreSQL
```

Mock only true external systems such as email delivery.

---

# 48. Test utilities

Create ergonomic helpers under:

```text
internal/platform/testutil
```

Desired developer experience:

```go
app := testutil.NewApp(t)

user := app.CreateUser(...)
client := app.AuthenticatedClient(user)

response := client.PostJSON("/v1/foo", request)
```

Test helpers should provide:

* isolated database state
* migrations
* fake email sender where appropriate
* user/account factories
* authenticated requests
* response decoding

Tests must not depend on execution order.

---

# 49. Auth integration tests

Must cover at least:

* signup succeeds
* duplicate signup handled
* invalid signup rejected
* unknown JSON fields rejected
* verification flow
* expired verification token
* reused verification token
* login succeeds
* invalid credentials
* logout
* logout-all
* forgot-password does not enumerate users
* reset succeeds
* reset token single-use
* reset invalidates previous sessions
* unauthenticated protected route
* malformed session cookie
* expired session
* rate limiting behavior

---

# 50. API wrapper tests

Test separately:

* valid JSON
* malformed JSON
* unknown field
* oversized body
* missing required field
* invalid email
* invalid path UUID
* invalid query parameter
* handler application error
* unexpected error
* correct HTTP status
* no internal error leakage
* request ID behavior

The handler wrapper is critical infrastructure and should have excellent test coverage.

---

# 51. Transaction helper tests

Test:

* commit
* rollback
* context cancellation
* returned callback error
* connection availability after transaction
* panic behavior if explicitly supported/documented

---

# 52. CI

Create GitHub Actions CI.

Run:

```text
go test ./...
go test -race ./...
go vet ./...
golangci-lint
```

and any required integration tests against PostgreSQL.

CI should also ensure:

* project builds
* migrations can be applied to an empty database

Keep CI understandable.

---

# 53. Formatting and linting

Use:

```text
gofmt
go vet
golangci-lint
```

Do not enable excessive stylistic lint rules that slow development without improving correctness.

Lint configuration should emphasize:

* correctness
* error handling
* resource safety
* suspicious code

---

# 54. Documentation

Create:

```text
README.md
docs/architecture.md
docs/api-conventions.md
docs/database.md
docs/testing.md
```

README should contain:

1. prerequisites
2. first startup
3. development commands
4. migrations
5. tests
6. local email testing
7. repository structure

Documentation must describe the actual implementation, not aspirational architecture.

---

# 55. Example feature

After platform infrastructure is complete, implement one intentionally simple example feature to prove the conventions.

For example:

```text
internal/example/
```

with:

```text
create.handler.go
example.repository.go
example.model.go
```

It should demonstrate:

* authenticated route
* account ownership
* JSON binding
* path/query binding where useful
* input validation
* repository query
* error handling
* integration test

The example must be easy to delete when starting a real project.

Do not turn it into a fake business domain.

---

# 56. Non-goals

The base template should NOT contain:

* billing
* Stripe
* subscription plans
* entitlement logic
* complex RBAC
* Kubernetes manifests
* Terraform
* Redis
* Kafka
* RabbitMQ
* event sourcing
* CQRS
* GraphQL
* generic event bus
* generic repository framework
* ORM
* frontend
* analytics platform
* feature flag platform

These can be added per product.

---

# 57. Implementation milestones

Implement in this order.

## M0 — Bootstrap

* initialize Go module
* directory structure
* config
* Makefile
* Compose
* PostgreSQL
* Mailpit
* basic Dockerfile
* CI skeleton

## M1 — Platform foundation

* logger
* database pool
* transaction helper
* migrations
* server lifecycle
* health/readiness
* middleware stack

## M2 — Typed HTTP layer

* generic handler function
* JSON binder
* path binder
* query binder
* strict JSON
* validator integration
* error model
* response encoder
* tests

M2 is a major quality gate.

Do not proceed until wrapper tests are solid.

## M3 — Identity model

* users
* accounts
* memberships
* sessions
* auth repositories

## M4 — Authentication

* signup
* verification
* login
* logout
* logout-all
* forgot password
* reset password
* auth middleware
* rate limiting

## M5 — Email and jobs

* email abstraction
* Mailpit integration
* job queue
* worker
* transactional enqueue
* email jobs

## M6 — Testing foundation

* test database lifecycle
* factories
* HTTP client
* auth helpers
* integration tests

## M7 — Example feature

Implement one small feature end-to-end using all conventions.

## M8 — Hardening

* race tests
* graceful shutdown tests where practical
* error/log review
* security review
* dependency review
* documentation
* CI finalization

---

# 58. Definition of done

The template is done when all of the following are true.

A new developer can run:

```bash
cp .env.example .env
make compose-up
make migrate
make dev
```

and obtain a functioning API.

They can:

1. sign up
2. receive a verification email in Mailpit
3. verify the account
4. log in
5. access an authenticated endpoint
6. log out
7. request password reset
8. reset password
9. log in with the new password

Additionally:

```bash
make test
make test-race
make lint
make vet
make build
```

must succeed.

A fresh PostgreSQL database must accept all migrations.

The production Docker image must build.

No code outside `old/` may import anything from `old/`.

The example feature must demonstrate the intended package-as-feature architecture.

---

# 59. Final architectural test

Before declaring completion, review the repository and answer:

1. Can a new feature be added without touching platform internals?
2. Does a normal handler mostly contain product logic rather than HTTP boilerplate?
3. Is SQL easy to find?
4. Are transactions explicit?
5. Can errors be traced internally without leaking implementation details?
6. Does input validation happen consistently without handler repetition?
7. Is tenant/account ownership explicit?
8. Can authentication be understood without hidden magic?
9. Can integration tests exercise real PostgreSQL easily?
10. Is every major abstraction simpler than repeating the code it replaces?

If an abstraction fails question 10, simplify it.
