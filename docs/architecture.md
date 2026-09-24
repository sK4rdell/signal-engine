## Purpose

This document describes the architectural conventions for the reusable Go backend template.

The architecture is intentionally simple.

The goal is not to create a framework.

The goal is to make product-specific development fast while keeping infrastructure concerns solved consistently.

---

# 1. Architectural principles

The codebase follows these principles:

1. package by feature
2. explicit dependencies
3. explicit SQL
4. thin HTTP handlers
5. business logic separated from transport and persistence
6. infrastructure shared only when genuinely cross-cutting
7. account ownership explicit
8. minimal abstraction
9. production-safe defaults
10. integration tests for real behavior

Prefer straightforward code over clever abstractions.

---

# 2. Top-level structure

Expected structure:

```text
cmd/
  api/
  worker/
  migrate/
  ingest/         manual, bounded source ingestion (no scheduler)

internal/
  app/            composition root shared by cmd/* and the test harness
  auth/
  account/
  example/        example feature, delete when starting a product
  publicevent/    domain: source observations, public events, their API
  source/
    <source>/     one package per public source: client, parser, ingester
  <product features>/

  platform/
    api/          typed handler wrapper, binding, validation, middleware, router
    apperror/
    config/
    database/
    email/
    jobs/
    logging/
    metrics/      in-process counters served at /metricsz
    password/     Argon2id hashing
    ratelimit/    in-memory fixed-window limiter
    server/       http.Server lifecycle, /healthz, /readyz
    testutil/     integration harness; pgtest/ holds the bare database lifecycle

migrations/       goose SQL, embedded into the binaries

docs/

old/
```

---

# 3. Package by feature

Application functionality is grouped by business capability.

Preferred:

```text
internal/
  auth/
  account/
  opportunity/
  company/
```

Avoid:

```text
internal/
  handlers/
  repositories/
  services/
  models/
```

The reason is locality.

When working on `opportunity`, most relevant code should be close together.

---

# 4. Feature package structure

A feature may contain:

```text
opportunity/
  opportunity.model.go
  opportunity.repository.go
  create.handler.go
  create.service.go
  list.handler.go
```

Not every feature needs every layer.

A simple feature may be:

```text
handler -> repository
```

A feature with meaningful application logic may be:

```text
handler -> service -> repository
```

Do not create a service merely to forward arguments.

---

# 5. Handler responsibilities

Handlers are responsible for transport concerns.

They receive already-bound and validated request data through the shared API layer.

Handlers may:

* access authenticated principal information
* call application/service logic
* call repositories for simple operations
* return typed responses
* return application errors

Handlers should not:

* contain SQL
* manually decode normal JSON requests
* implement repeated validation
* expose database errors
* contain complex business rules

---

# 6. Service/application logic

A service exists when an operation contains meaningful orchestration or business logic.

Examples:

```text
create account + membership + verification token
change opportunity state
create resource + enqueue asynchronous work
perform multi-repository transaction
```

A service should describe an application operation.

Avoid large god-services.

Prefer feature-specific services.

---

# 7. Repository responsibilities

Repositories own persistence details.

Repositories:

* execute SQL
* scan rows
* map database representations
* interpret expected database conditions
* expose feature-oriented persistence operations

Repositories do not:

* own HTTP concerns
* write responses
* perform authorization based solely on client input
* start arbitrary transactions unless explicitly required

Repository methods should accept `context.Context`.

---

# 8. Platform packages

`internal/platform` contains infrastructure genuinely shared across features.

Examples:

```text
platform/api
platform/database
platform/logging
platform/email
platform/jobs
platform/config
platform/apperror
```

Avoid generic packages like:

```text
utils
common
helpers
misc
```

If code is only used by one feature, keep it in that feature.

---

# 9. Dependency direction

Normal dependency direction:

```text
cmd
 ↓
features
 ↓
platform primitives
```

Feature packages should not depend on `cmd`.

Platform packages must not depend on product features.

Avoid circular dependencies.

---

# 10. Application startup

`internal/app` is the composition root: it builds repositories, services,
the router (with every route registered) and the job worker (with every
handler registered) from a `Deps` value holding config, logger, pool,
metrics and the email sender. `cmd/api`, `cmd/worker` and
`testutil.NewApp` all call `app.New`, so tests exercise exactly the wiring
production runs. The `cmd/*` binaries only load configuration, open
infrastructure, call `app.New` and manage process lifecycle.

Typical startup flow:

```text
load config
  ↓
initialize logger
  ↓
open PostgreSQL pool (verified with a ping)
  ↓
build email sender
  ↓
app.New: repositories, services, job handlers, routes
  ↓
optionally start the worker in-process (JOBS_WORKER_IN_API)
  ↓
start HTTP server
  ↓
SIGTERM/SIGINT: stop accepting, drain requests (HTTP_SHUTDOWN_TIMEOUT),
stop worker (JOBS_SHUTDOWN_TIMEOUT), close pool
```

Dependency construction should be explicit.

Do not introduce a dependency injection framework.

---

# 11. Worker startup

`cmd/worker` is responsible for:

```text
load config
  ↓
initialize logger
  ↓
open PostgreSQL pool
  ↓
register job handlers
  ↓
start worker loop
  ↓
graceful shutdown
```

The worker uses the same feature/application code as the API: it calls
`app.New` and runs the returned worker, whose handlers were registered by
the feature packages (`auth.RegisterJobs`).

---

# 12. Tenant/account model

The base ownership model is:

```text
users
accounts
account_memberships
```

Product resources normally belong to:

```text
account_id
```

not directly to `user_id`.

This allows:

* multiple users per account
* future teams
* future invitations
* cleaner authorization boundaries

---

# 13. Authorization principle

Authentication answers:

> Who is this user?

Authorization answers:

> May this user perform this action on this account/resource?

Never trust client-provided ownership identifiers without verification.

Prefer database queries that scope account-owned resources directly:

```sql
SELECT ...
FROM opportunities
WHERE id = $1
  AND account_id = $2;
```

rather than:

```text
load resource
then later remember to verify ownership
```

This reduces accidental cross-tenant leakage.

---

# 14. Authentication architecture

Browser authentication uses opaque server-side sessions.

Flow:

```text
secure random token
  ↓
token returned in HttpOnly cookie
  ↓
hash stored in PostgreSQL
```

The raw session token never needs to be recoverable from the database.

`auth.RequireSession` resolves the session cookie and stores an
`api.Principal{UserID, SessionID}` in the request context (the principal
type lives in `platform/api` so that feature packages can read it without
importing `auth`). `account.RequireMembership` then resolves the
`{accountID}` route parameter, verifies membership and stores
`account.Context{AccountID, Role}`. Both add `user_id` / `account_id` to the
request log line.

Convention for authorization failures: a request for an account the caller
is not a member of answers `403 account_membership_required` (account IDs
are not secrets). Resources inside an account are always queried with
`WHERE id = $1 AND account_id = $2`, so a resource in another account
answers the same tenant-safe `404` as a nonexistent one.

---

# 15. Asynchronous work

Slow or retryable external work should not normally execute inside HTTP requests.

Examples:

* email
* external enrichment
* webhook delivery
* report generation

Use the PostgreSQL-backed job queue (`platform/jobs`).

Where state mutation and job scheduling belong together, enqueue the job in
the same transaction: `jobs.Enqueue(ctx, tx, ...)`. Signup does exactly this
for the verification email. Job handlers are owned by the feature that
enqueues the job (`auth/jobs.go`) and registered on the worker in
`internal/app`.

---

# 16. External dependencies

External systems should normally sit behind small interfaces near their consumers.

Examples:

```go
type Sender interface {
    Send(context.Context, Message) error
}
```

Do not create interfaces for every internal type.

Interfaces exist at useful boundaries.

---

# 17. Errors

Internal errors preserve their cause.

Application errors may contain:

* public code
* public message
* HTTP status
* cause
* field errors

The HTTP boundary performs final conversion.

Unexpected internal errors must never expose implementation details to clients.

---

# 18. Logging

Logging is structured using `log/slog`.

Request context should attach fields such as:

```text
request_id
user_id
account_id
```

Feature code should add domain-specific identifiers.

Errors should generally be logged at system boundaries rather than repeatedly at each layer.

---

# 19. No premature abstractions

Do not create:

* generic repository framework
* generic service framework
* domain event framework
* event bus
* CQRS infrastructure
* plugin architecture

until a real product requires them.

Prefer some duplication to the wrong shared abstraction.

---

# 19b. Sources and public events

Source adapters live under `internal/source/<source>` and own every
source-specific concern (requests, pagination, parsing, identifiers,
normalisation). They depend on `internal/publicevent`, never the other way
round, and never on each other. `publicevent` stores what a source showed
us (append-only observations) separately from our canonical reading
(events). See `docs/public-events.md`.

---

# 20. `old/`

`old/` contains previous Bolagsbild code.

It is archival/reference material.

It may be studied.

It must not become a runtime dependency.

New architectural decisions are governed by:

```text
CLAUDE.md
TEMPLATE_SPEC.md
docs/*
```

not `old/`.

---

# 21. Architectural success criterion

The architecture is successful if a new feature can usually be implemented by adding a feature package without changing platform internals.

A typical product feature should be understandable by reading:

```text
handler
service if present
repository
model
tests
```

without needing to understand a custom framework.

