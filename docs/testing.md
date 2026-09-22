## Purpose

This document defines the testing strategy for the backend template.

Testing exists to enable fast change with confidence.

The goal is not maximum test count or coverage percentage.

The goal is high confidence in behavior that matters.

---

# 1. Core philosophy

Tests are part of implementation.

A feature is not complete merely because:

* it compiles
* the happy path worked manually
* Claude says it should work

Relevant automated tests must exist and be run.

Prefer testing observable behavior over implementation details.

---

# 2. Test pyramid for this repository

The repository uses three main categories.

## Unit tests

Use for pure logic.

Examples:

* calculations
* parsers
* transformations
* state transition rules
* deterministic matching logic

These should be fast and require no infrastructure.

## Repository integration tests

Use real PostgreSQL.

Examples:

* queries
* scanning
* uniqueness
* account scoping
* transactions
* pagination
* locking

## HTTP integration tests

Preferred for normal API features.

Exercise:

```text
HTTP request
 ↓
router
 ↓
middleware
 ↓
typed handler wrapper
 ↓
application logic
 ↓
repository
 ↓
PostgreSQL
```

These provide the highest value for most product features.

---

# 3. Mocking policy

Mock as little as practical.

Good things to fake:

* external email provider
* external HTTP API
* payment gateway
* object storage provider

Normally do not mock:

* repositories
* PostgreSQL
* router
* handler wrapper
* authentication middleware

If a real internal component is cheap to run in tests, use the real component.

---

# 4. PostgreSQL only

Database integration tests run against PostgreSQL.

Never use SQLite as a lightweight replacement.

Reasons include differences in:

* SQL syntax
* constraints
* locking
* transactions
* concurrency
* query planner behavior
* PostgreSQL-specific types/features

---

# 5. Test database isolation

Tests must not depend on execution order.

A database-backed test receives isolated state.

The infrastructure (`testutil/pgtest`) does:

```text
test PostgreSQL instance (TEST_DATABASE_URL)
  ↓
once per migration set: create a template database and migrate it
  ↓
per test: CREATE DATABASE t_<random> TEMPLATE tmpl_<hash>
  ↓
test
  ↓
t.Cleanup: close the pool (failing on leaked connections), DROP DATABASE
```

Copying a template is far cheaper than migrating per test, and every test
still gets a fully isolated database. An advisory lock serialises database
creation across parallel test processes. Tests skip when `TEST_DATABASE_URL`
is unset; `make test` sets it to the compose database.

Do not universally wrap tests in transactions.

Transaction-per-test would make it harder to correctly test:

* commits
* rollback
* workers
* multiple connections
* concurrent transactions
* job visibility after commit

---

# 6. Test infrastructure

Provide:

```text
internal/platform/testutil
```

Desired usage:

```go
func TestCreateFoo(t *testing.T) {
    app := testutil.NewApp(t)

    user := app.CreateUser()
    client := app.AuthenticatedClient(user)

    res := client.PostJSON("/v1/foo", map[string]any{
        "name": "Example",
    })

    res.AssertStatus(t, http.StatusCreated)
}
```

The test harness should remove repetitive setup while keeping behavior visible.

---

# 7. Test application helper

`testutil.NewApp(t)` calls the production composition root (`app.New`) on an
isolated database with:

* a test configuration (`ENV=test`, allowed origin `http://app.test`,
  generous rate limits) adjustable through `testutil.WithConfig`
* a JSON logger captured in memory, readable with `app.Logs()` for
  "never logged" assertions
* `app.Email`, an `email.Recorder` in place of SMTP
* `app.Worker`, with `app.RunJobs()` executing every runnable job once
* `app.Jobs(type)` to inspect queued jobs
* `app.Auth`, the real auth service, used by `AuthenticatedClient`

Keep this helper deterministic.

---

# 8. Factories

Provide small factories for common setup.

Examples:

```go
user := app.CreateUser()
account := app.CreateAccount()
app.AddMembership(user, account, "owner")
```

Factories should create valid defaults.

Allow overrides when scenarios require specific state.

Do not build a large fixture framework.

---

# 9. Authenticated client

Tests should make authenticated API usage easy.

Example:

```go
client := app.AuthenticatedClient(user)
```

This helper should use the same session behavior as production rather than bypassing auth middleware.

`app.AuthenticatedClient(user)` creates a session through the real auth
service and stores the resulting cookie. The client sends the allowed
`Origin` header on every request (set `client.Origin` to test the CSRF
guard), keeps cookies between requests like a browser, and uses a fixed
`RemoteAddr` (change it to test per-IP rate limits).

---

# 10. Response helpers

Provide lightweight helpers for:

* HTTP status assertion
* JSON decoding
* error response decoding
* response header inspection

Avoid building a full assertion framework.

Use standard `testing` by default.

---

# 11. Table-driven tests

Use table-driven tests when cases share the same structure.

Example:

```go
tests := []struct {
    name       string
    email      string
    wantStatus int
}{
    {
        name:       "valid",
        email:      "foo@example.com",
        wantStatus: http.StatusCreated,
    },
    {
        name:       "invalid email",
        email:      "foo",
        wantStatus: http.StatusUnprocessableEntity,
    },
}
```

Do not force complex scenarios into tables when separate tests are easier to understand.

---

# 12. Feature testing checklist

For each endpoint, consider:

```text
[ ] happy path
[ ] malformed request
[ ] field validation
[ ] authentication
[ ] authorization
[ ] cross-account isolation
[ ] not found
[ ] conflict
[ ] domain failure
[ ] persisted state
[ ] asynchronous side effect
[ ] public error code
```

Use judgment.

Do not duplicate infrastructure-level tests in every feature.

---

# 13. Tenant isolation

Account isolation is a critical invariant.

Example test:

```text
account A creates resource X

account B requests X

expected:
no data leakage
```

Test applicable operations:

* get
* update
* delete
* list
* nested resources

Prefer tenant-safe repository queries.

---

# 14. Typed handler wrapper tests

The handler wrapper requires dedicated comprehensive tests.

Cover:

## JSON

* valid body
* malformed JSON
* unknown field
* empty body
* multiple JSON documents
* oversized body

## Path

* valid parameter
* malformed UUID
* missing parameter behavior

## Query

* valid string
* valid bool
* valid integer
* malformed values
* repeated values if supported

## Validation

* required fields
* email
* range
* enum

## Handler execution

* success
* application error
* unexpected error

## Response

* status
* content type
* JSON
* error format

## Context

* request ID
* cancellation where meaningful

---

# 15. Error handling tests

Verify:

* internal error details do not leak
* stable public code
* expected HTTP status
* validation field errors
* not-found behavior
* conflict behavior

Avoid asserting exact internal log strings unless logging itself is being tested.

---

# 16. Transaction tests

Use real PostgreSQL.

Cover:

* callback success commits
* callback failure rolls back
* original error returned
* connection remains usable
* context cancellation
* no connection leak

Test commit failure only where it can be reproduced meaningfully without fragile hacks.

---

# 17. Signup tests

Cover:

* successful signup
* normalized email
* invalid email
* password too short
* duplicate email
* user created
* account created
* owner membership created
* verification token created
* verification job queued

Verify transaction atomicity where practical.

---

# 18. Email verification tests

Cover:

* valid token
* invalid token
* expired token
* reused token
* successful verification
* resend
* resend throttling

Verify plaintext token is not stored.

---

# 19. Login tests

Cover:

* correct credentials
* wrong password
* unknown email
* equivalent public failure behavior
* session created
* secure cookie semantics where testable
* raw session token not stored in database
* rate limiting

---

# 20. Session tests

Cover:

* valid session
* missing cookie
* invalid session
* expired session
* revoked session
* logout
* logout-all

Test that logout-all invalidates multiple active sessions.

---

# 21. Password reset tests

Cover:

* request for existing user
* request for unknown user
* public response does not enumerate accounts
* valid reset
* invalid token
* expired token
* reused token
* old password no longer works
* new password works
* existing sessions invalidated

---

# 22. Security tests

Actively test misuse cases.

Examples:

* user enumeration
* replay
* expired tokens
* brute force protection
* malformed cookies
* untrusted Origin
* cross-account access
* oversized request
* sensitive error leakage

Security-critical behavior deserves explicit tests.

---

# 23. Job queue tests

Use real PostgreSQL.

Cover:

* enqueue
* enqueue in transaction
* rollback removes pending job
* commit makes job available
* worker claim
* success
* failure
* retry
* backoff
* max attempts
* future scheduling
* unknown job type
* concurrent workers
* graceful cancellation

The concurrency test should demonstrate that two workers cannot execute the same claimed job simultaneously.

---

# 24. Email job flow

Test full behavior:

```text
signup request
  ↓
database state committed
  ↓
job exists
  ↓
worker executes job
  ↓
fake sender captures email
```

This gives confidence in the asynchronous flow without requiring an actual
SMTP provider. The SMTP sender itself has an integration test against the
compose Mailpit that runs when `TEST_SMTP_ADDR` and `TEST_MAILPIT_URL` are
set (the Makefile and CI set them).

---

# 25. Migration tests

CI must apply all migrations to an empty PostgreSQL database.

This ensures:

```text
fresh repository
+
migrations
=
current schema
```

Migrations are part of application behavior.

---

# 26. Health tests

`/healthz` should succeed without requiring dependent services.

`/readyz` should reflect database readiness.

Test both healthy and unavailable dependency scenarios where practical.

---

# 27. Middleware tests

Critical middleware should have focused tests.

Examples:

* request ID
* recovery
* security headers
* Origin/CSRF protection
* CORS behavior
* client IP/trusted proxy behavior
* authentication

Do not over-test trivial framework behavior.

---

# 28. Regression tests

Bug fixes should normally include regression tests.

Workflow:

```text
reproduce failure
  ↓
test fails
  ↓
fix code
  ↓
test passes
```

Keep the test.

---

# 29. Race tests

Run:

```bash
go test -race ./...
```

before milestone completion.

Concurrency-sensitive code should also receive targeted package-level race runs during development.

Examples:

* workers
* in-memory rate limiter
* shared caches/state

---

# 30. Flaky tests

A flaky test is a defect.

Do not fix flakiness by adding arbitrary sleeps.

Use:

* explicit synchronization
* bounded polling
* durable database state
* deterministic clocks/hooks when justified

Async tests must have bounded failure time.

---

# 31. Test timing

During implementation:

```text
change
 ↓
narrow relevant test
 ↓
continue
```

Examples:

```bash
go test ./internal/platform/api/...
go test ./internal/auth/...
```

At milestone completion:

```bash
make test
make test-race
```

---

# 32. CI

CI should run at minimum:

```bash
go test ./...
go test -race ./...
go vet ./...
golangci-lint
```

CI should also:

* build the binaries
* start PostgreSQL where needed
* apply migrations from zero

No external SaaS dependency should be required for CI unless unavoidable.

---

# 33. Coverage

Do not enforce an arbitrary global coverage percentage.

Coverage can help identify suspicious gaps.

It is not itself a success metric.

Prefer tests protecting important behavior over tests targeting lines.

---

# 34. Test quality

Good tests:

* explain the scenario
* assert important behavior
* fail for useful reasons
* tolerate internal refactoring

Avoid tests tightly coupled to:

* private helper calls
* internal function order
* incidental implementation details

---

# 35. Test naming

Prefer names like:

```go
func TestResetPassword_InvalidatesExistingSessions(t *testing.T)
func TestGetOpportunity_CannotReadOtherAccount(t *testing.T)
func TestHandle_RejectsUnknownJSONField(t *testing.T)
```

Names should communicate behavior.

---

# 36. Definition of tested

A feature is sufficiently tested when:

* core happy path works through the actual stack
* important failure modes are protected
* security/account boundaries are covered
* SQL has run against PostgreSQL
* side effects are verified
* relevant tests have actually been executed

Testing should increase confidence, not ceremony.
