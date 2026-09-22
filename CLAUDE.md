# CLAUDE.md

This file defines the non-negotiable engineering rules for this repository.

For implementation requirements, read:

* `TEMPLATE_SPEC.md`
* `docs/architecture.md`
* `docs/api-conventions.md`
* `docs/database.md`
* `docs/testing.md`

These documents define the intended architecture. Do not invent parallel conventions.

---

# 1. Primary goal

This repository is a reusable Go backend template optimized for rapid product validation.

Optimize for:

1. correctness
2. clarity
3. development speed
4. maintainability
5. production-safe defaults

Prefer boring, explicit, idiomatic Go.

Do not optimize prematurely for hypothetical scale.

Do not turn the template itself into a framework or product.

---

# 2. `old/` is reference material only

The previous Bolagsbild implementation lives under:

```text
old/
```

It may be inspected for:

* useful SQL/query patterns
* prior implementations
* lessons learned
* small utilities worth adapting

But:

* `old/` is NOT the architectural source of truth.
* Never import code from `old/`.
* Never create runtime dependencies on `old/`.
* Never mass-copy files from `old/`.
* Never preserve a pattern just because it already exists there.
* Do not modify `old/` unless explicitly instructed.

Reuse ideas only when they fit the new architecture.

The new specification always wins.

---

# 3. Package by feature

Business/application code belongs in feature packages.

Preferred:

```text
internal/
  auth/
  account/
  opportunity/
  company/
```

Do NOT organize application code globally as:

```text
handlers/
repositories/
services/
models/
```

Shared technical infrastructure belongs under:

```text
internal/platform/
```

---

# 4. Feature conventions

Prefer feature-local files such as:

```text
foo.handler.go
foo.repository.go
foo.service.go
foo.model.go
```

Do not split tiny features into unnecessary layers.

This is valid:

```text
handler -> repository
```

when business logic is trivial.

Use:

```text
handler -> service -> repository
```

when meaningful application logic exists.

Do NOT create empty service layers purely for architectural symmetry.

---

# 5. Handlers are transport adapters

Handlers may:

* consume typed validated input
* read auth/context
* call application/domain logic
* return typed responses
* return application errors

Handlers must NOT:

* contain SQL
* manually decode normal JSON requests
* duplicate ordinary validation
* manually map errors to HTTP
* manually serialize normal responses
* contain substantial business logic

Use the shared typed handler wrapper.

---

# 6. Typed HTTP wrapper

Normal handlers should use the repository's typed handler abstraction.

Conceptually:

```go
func CreateFoo(service *Service) api.HandlerFunc[CreateFooRequest, CreateFooResponse] {
    return func(
        ctx context.Context,
        req CreateFooRequest,
    ) (CreateFooResponse, error) {
        ...
    }
}
```

The wrapper owns normal:

* body binding
* path binding
* query binding
* input validation
* response serialization
* application error mapping

Do not bypass the wrapper without a concrete reason.

Reasonable exceptions include:

* file downloads
* streaming
* webhooks requiring raw-body verification
* protocol-specific behavior

Document the reason when bypassing it.

---

# 7. Input validation

Structural validation belongs at the HTTP boundary.

Examples:

* required fields
* email format
* UUID format
* string lengths
* numeric bounds
* enums
* simple cross-field validation

Business validation belongs in application/domain logic.

Examples:

* email already exists
* invalid state transition
* resource belongs to another account
* account is not allowed to perform an action

Do not perform database-backed checks inside validator functions.

Use strict JSON decoding.

Reject:

* malformed JSON
* unknown fields
* multiple JSON documents
* oversized request bodies

Client mistakes must not be silently ignored.

---

# 8. Database

Use:

```text
pgx/v5
pgxpool
```

Do not introduce:

* GORM
* Ent
* sqlx
* another ORM/query abstraction

without explicit approval.

Keep SQL explicit and feature-local.

Repositories should normally accept the shared `database.DBTX` abstraction when transaction participation is needed.

Do not hide SQL behind generic repository frameworks.

---

# 9. Transactions

Use the shared transaction helper.

Prefer:

```go
database.InTx(ctx, pool, func(tx pgx.Tx) error {
    ...
})
```

Do not:

* manually acquire pooled connections for ordinary transactions
* spread transaction lifecycle management across multiple layers
* start transactions inside repositories without a specific reason

Keep transaction boundaries around complete application operations.

See `docs/database.md`.

---

# 10. Migrations

Use goose SQL migrations.

Schema changes require migrations.

Never rely on application startup to create production schema.

Never modify an already-applied migration to represent a new schema state.

Create a new migration.

---

# 11. Errors

Use the shared application error package.

Public API errors must have stable machine-readable codes.

Do not expose internal errors directly to clients.

Never do:

```go
http.Error(w, err.Error(), ...)
```

for internal failures.

Unexpected internal errors should return a generic public error while preserving the original cause internally.

---

# 12. Log errors once

Do not log the same error at every layer.

Normal pattern:

```text
repository -> return
service    -> wrap/add context
handler    -> return
HTTP boundary -> log
```

For background work:

```text
job -> return
worker boundary -> log
```

Log earlier only when the local event itself is independently meaningful.

---

# 13. Logging

Use:

```text
log/slog
```

Use structured fields.

Prefer:

```go
logger.Info(
    "company created",
    "company_id", company.ID,
)
```

Do not introduce another logging framework.

Never log:

* passwords
* password hashes
* session tokens
* cookies
* authorization headers
* reset tokens
* verification tokens
* API secrets
* database credentials

Be cautious with full request logging.

---

# 14. Authentication

Use first-party opaque server-side sessions.

Do not replace browser sessions with JWT unless explicitly instructed.

Session tokens must:

* be cryptographically random
* be stored hashed in PostgreSQL
* never be logged

Auth cookies must use secure production settings.

Password reset should invalidate existing sessions.

---

# 15. Passwords and security tokens

Use Argon2id through the centralized password package.

Do not implement password hashing inside feature code.

Security tokens such as:

* email verification tokens
* password reset tokens

must:

* use cryptographically secure randomness
* expire
* be single-use
* be stored hashed
* never be logged

Do not use UUIDs as security tokens.

---

# 16. Accounts and tenancy

Customer-owned resources should normally belong to:

```text
account_id
```

not directly to:

```text
user_id
```

Never trust a client-supplied `account_id` without authorization checks.

Avoid accidental cross-tenant access.

Account-scoped repository queries should make tenancy explicit.

Cross-tenant data leakage is a critical defect.

---

# 17. Email

Application code depends on the shared email abstraction, not directly on a provider.

Email should normally be sent asynchronously through background jobs.

Do not make external SMTP/API calls while holding a database transaction.

Where state changes and email scheduling belong together, enqueue the job transactionally.

---

# 18. Background jobs

Use the existing PostgreSQL-backed job mechanism.

Do not introduce:

* Kafka
* RabbitMQ
* Redis queues
* Temporal

for ordinary background work without a demonstrated need.

Jobs must be retry-safe.

Use PostgreSQL locking correctly for concurrent workers.

---

# 19. Configuration

Configuration belongs in the typed config package.

Do not scatter:

```go
os.Getenv(...)
```

through application code.

Required production configuration must fail fast during startup.

Secrets must never be logged.

---

# 20. Dependencies

Before adding a dependency, ask:

1. Does the standard library already solve this clearly?
2. Is this a security/correctness-sensitive commodity problem?
3. Does the dependency materially simplify the code?
4. Is it actively maintained?
5. Does it fit the existing architecture?

Do not add dependencies for trivial helpers.

Do not reimplement security-sensitive primitives such as:

* password hashing algorithms
* structural validation libraries

---

# 21. Shared packages

Do not create dumping grounds such as:

```text
utils/
helpers/
common/
misc/
```

Shared packages must have clear responsibilities.

Good:

```text
platform/api
platform/database
platform/email
platform/logging
platform/apperror
```

Bad:

```text
platform/utils
```

---

# 22. Interfaces

Do not define interfaces preemptively.

Prefer concrete types until an interface has a real consumer.

Good reasons for interfaces:

* true external boundary
* actual multiple implementations
* test double for an external dependency
* shared DBTX abstraction

Bad reason:

> "We might need another implementation someday."

---

# 23. Testing is part of implementation

Tests are not cleanup work.

Do not report behavior complete until relevant automated tests exist and have been run successfully.

Default testing approach:

* pure logic -> unit tests
* SQL -> integration tests against real PostgreSQL
* API features -> integration tests through the real router/middleware/handler/repository stack

Do not use SQLite as a substitute for PostgreSQL.

Mock/fake only true external boundaries such as:

* email providers
* external HTTP APIs
* payment providers
* object storage

Do not mock repositories merely to avoid exercising SQL.

See `docs/testing.md`.

---

# 24. Feature test requirements

For each new API feature, test applicable behavior such as:

```text
[ ] happy path
[ ] malformed input
[ ] validation failures
[ ] authentication
[ ] authorization
[ ] cross-account isolation
[ ] important domain failures
[ ] not found
[ ] conflicts
[ ] persisted state
[ ] side effects
[ ] stable public error codes
```

Not every feature needs every case.

Use judgment.

Do not duplicate platform-level tests unnecessarily.

---

# 25. Critical infrastructure requires stronger tests

Treat these as critical:

* typed handler wrapper
* transaction helper
* authentication
* session handling
* token flows
* account isolation
* background job queue

They require more extensive edge-case and failure testing than ordinary CRUD.

Hard quality gates:

## Typed HTTP layer

Do not consider it complete until tests cover:

* body binding
* path binding
* query binding
* strict JSON
* validation
* application errors
* unexpected errors
* response serialization
* request IDs

## Authentication

Do not consider it complete until integration tests cover:

* signup
* email verification
* login
* logout
* logout-all
* forgot password
* password reset
* token expiry
* token reuse
* session invalidation
* enumeration resistance
* rate limiting
* account authorization

---

# 26. PostgreSQL test isolation

Database-backed tests must be isolated.

Tests must not depend on execution order or stale state.

Use the shared test infrastructure to create isolated database/schema state and apply real migrations.

Do not use transaction-per-test as the universal isolation strategy because it can hide real behavior involving:

* commits
* rollbacks
* multiple connections
* workers
* concurrency

The test infrastructure should make integration tests easy enough to write frequently.

---

# 27. Test utilities

Use:

```text
internal/platform/testutil
```

for ergonomic integration testing.

The desired experience is approximately:

```go
app := testutil.NewApp(t)

user := app.CreateUser()
client := app.AuthenticatedClient(user)

res := client.PostJSON("/v1/foo", request)
res.AssertStatus(t, http.StatusCreated)
```

Helpers may provide:

* isolated PostgreSQL state
* migrations
* test config
* router
* repositories/services
* fake email sender
* account/user factories
* authenticated client
* response helpers
* job inspection helpers

Do not hide important test behavior behind excessive helper magic.

---

# 28. Bug fixes

When fixing a bug:

1. reproduce it with a test when practical
2. fix the underlying cause
3. keep the regression test

Do not merely patch symptoms.

If a regression test is impractical, explain why.

---

# 29. Race and concurrency safety

Assume:

* HTTP handlers run concurrently
* workers run concurrently
* multiple process replicas may eventually exist

Avoid mutable package-global state.

Run:

```bash
go test -race ./...
```

before major milestones and before final completion.

Do not solve flaky asynchronous tests with arbitrary sleeps.

Use explicit synchronization or bounded polling.

---

# 30. Context

Pass `context.Context` through operations involving:

* HTTP lifecycle
* database access
* jobs
* external calls

Respect cancellation.

Do not store contexts in structs.

Use `context.Background()` only at true process roots.

---

# 31. Documentation

Keep implementation and documentation aligned.

When changing an architectural convention, update the relevant documentation in the same change.

Do not leave aspirational documentation describing behavior that does not exist.

---

# 32. Keep the template small

Before adding a subsystem or abstraction ask:

> Will most product experiments actually benefit from this?

If not, leave it out.

The base template intentionally excludes unless explicitly requested:

* Stripe
* billing
* complex RBAC
* Redis
* Kafka
* GraphQL
* Kubernetes
* Terraform
* event sourcing
* CQRS
* generic event bus
* frontend

---

# 33. Avoid speculative abstractions

Do not generalize code merely because it might be reusable later.

A useful rule:

> Prefer duplication over the wrong abstraction.

Shared abstractions should usually emerge after at least two real consumers exist.

Infrastructure primitives are exceptions when explicitly defined by the template specification.

---

# 34. Work autonomously

When implementing `TEMPLATE_SPEC.md`, proceed independently.

Do not repeatedly ask for confirmation about small decisions already constrained by the specification.

When several implementations would satisfy the requirements:

1. choose the simplest idiomatic Go approach
2. implement it
3. test it
4. document any non-obvious tradeoff

Ask for user input only when the decision materially changes:

* public API behavior
* security model
* architecture
* product semantics
* major dependencies
* project scope

Do not stop because an unimportant detail was unspecified.

---

# 35. Validate continuously

After meaningful changes, run the narrowest relevant checks.

Examples:

```bash
go test ./internal/platform/api/...
go test ./internal/auth/...
go test ./internal/opportunity/...
go vet ./...
```

Run integration tests when database/API behavior changes.

Run race tests for concurrency-sensitive changes where practical.

Do not accumulate large amounts of untested code.

---

# 36. Definition of done for a change

A change is not complete merely because it compiles.

Before reporting completion:

1. implementation is finished
2. relevant tests exist
3. relevant tests were run
4. tests pass
5. formatting passes
6. vet/lint relevant to the change pass
7. migrations were exercised when schema changed
8. documentation was updated when needed

If something could not be run, say so explicitly.

Never claim a check passed if it was not run.

---

# 37. Before milestone completion

Run the full project checks:

```bash
make fmt
make vet
make lint
make test
make test-race
make build
```

For database-related milestones also verify migrations against a clean PostgreSQL database.

Do not report a milestone complete while known failures remain.

---

# 38. Completion reports

When completing substantial work, report concisely:

1. what was implemented
2. important architectural decisions
3. tests/checks run
4. results
5. anything intentionally deferred
6. known limitations

Do not use vague statements such as:

> "should work"

when verification was possible.

---

# 39. Final rule

Prefer boring, explicit, tested Go.

The template succeeds when new product features can be built quickly without reopening solved infrastructure decisions.

The architecture should remove repetitive work without hiding program flow.

Every abstraction must earn its existence.
