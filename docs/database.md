
## Purpose

This document defines PostgreSQL, repository, migration and transaction conventions.

The database layer should remain explicit and predictable.

---

# 1. PostgreSQL

PostgreSQL is the only supported relational database.

Do not introduce SQLite as an alternative implementation.

Development and tests should use PostgreSQL behavior.

---

# 2. Driver

Use:

```text
github.com/jackc/pgx/v5
github.com/jackc/pgx/v5/pgxpool
```

Use pgx directly.

Do not add:

* GORM
* Ent
* sqlx
* database/sql wrappers

without explicit architectural approval.

---

# 3. Connection pool

The application owns one `pgxpool.Pool` per database.

Configuration should include sensible settings for:

* MaxConns
* MinConns where useful
* MaxConnLifetime
* MaxConnIdleTime
* HealthCheckPeriod

Startup should verify connectivity.

Shutdown should close the pool after request/worker shutdown.

---

# 4. DBTX

Repositories should work with both:

```text
*pgxpool.Pool
pgx.Tx
```

through a minimal shared interface.

Conceptually:

```go
type DBTX interface {
    Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
    Query(context.Context, string, ...any) (pgx.Rows, error)
    QueryRow(context.Context, string, ...any) pgx.Row
}
```

Only include methods actually needed.

Do not grow this into a generic database abstraction.

---

# 5. Repository convention

Repositories are feature-local.

Example:

```text
internal/opportunity/opportunity.repository.go
```

This template uses the second form consistently: repositories are
stateless types whose methods take the `DBTX` to run on:

```go
func (r *Repository) Create(ctx context.Context, db database.DBTX, ...) (Example, error)
```

Services pass the pool for single statements and the transaction from
`database.InTx` when several writes must succeed together. Constraint
violations are translated with `database.IsUniqueViolation(err, "constraint_name")`.

---

# 6. Explicit SQL

Keep SQL visible.

Example:

```go
const getOpportunitySQL = `
    SELECT
        id,
        account_id,
        name,
        created_at
    FROM opportunities
    WHERE id = $1
      AND account_id = $2
`
```

Queries should be formatted and readable.

Never interpolate untrusted values into SQL.

---

# 7. Dynamic queries

For dynamic filters, values remain parameterized.

Dynamic identifiers such as:

* sort columns
* sort direction

must come from explicit whitelists.

Example:

```go
switch sort {
case "created_at":
    orderColumn = "created_at"
case "name":
    orderColumn = "name"
default:
    return invalidSort
}
```

Never directly copy arbitrary query parameters into SQL.

---

# 8. Row handling

Always close returned rows:

```go
rows, err := db.Query(ctx, query)
if err != nil {
    ...
}
defer rows.Close()
```

Check:

```go
rows.Err()
```

after iteration.

Prefer `QueryRow` for single-row operations.

---

# 9. Not found

Translate expected PostgreSQL conditions close to the persistence boundary where useful.

Example:

```go
if errors.Is(err, pgx.ErrNoRows) {
    return ..., apperror.NotFound(...)
}
```

Do not expose pgx-specific errors through the HTTP API.

---

# 10. Constraints

Use database constraints for real invariants.

Examples:

* unique normalized email
* foreign keys
* non-null requirements
* useful check constraints

Do not rely solely on application pre-checks for uniqueness.

The database is the final concurrency-safe enforcement point.

---

# 11. Transactions

Use the shared transaction helper.

Desired behavior:

```go
err := database.InTx(ctx, pool, func(tx pgx.Tx) error {
    ...
    return nil
})
```

The helper must:

* begin transaction
* run callback
* rollback on callback failure
* commit only after callback succeeds
* preserve useful original errors
* always release the underlying connection

`database.InTx` returns the callback's error unchanged (usable with
`errors.Is`/`errors.As`), rolls back and re-raises when the callback
panics, and answers a `context.Canceled` error when the context is
cancelled mid-transaction. These behaviours are covered by
`internal/platform/database/tx_test.go`.

---

# 12. Transaction boundaries

Transactions belong around complete application operations.

Example:

```text
signup:
  create user
  create account
  create membership
  create verification token
  enqueue verification job

COMMIT
```

All of those database mutations either succeed together or fail together.

---

# 13. External calls inside transactions

Do not perform network calls while holding database transactions.

Bad:

```text
BEGIN
insert user
send email over network
COMMIT
```

Use:

```text
BEGIN
insert user
insert email job
COMMIT

worker sends email
```

This keeps transactions short and consistent.

---

# 14. Repository transaction participation

Repositories must not assume they own transaction lifecycle.

The application/service layer chooses whether to call the repository with:

```text
pool
```

or:

```text
tx
```

This allows repositories to compose safely.

---

# 15. Migrations

Use:

```text
github.com/pressly/goose/v3
```

Prefer SQL migrations.

Files live in:

```text
migrations/
```

---

# 16. Migration rules

Never edit an already-applied migration to represent a new schema state.

Create another migration.

Migrations should be understandable on their own.

Use descriptive names. `make migration name=create_widgets` scaffolds a
timestamped file with `Up` and `Down` sections.

Example:

```text
20260922090000_create_users.sql
20260922090100_create_accounts.sql
```

The files are embedded into the binaries (`migrations.FS`), so a deployed
image carries its schema and the test harness can migrate throwaway
databases without access to the source tree.

---

# 17. Migration execution

Production API startup must not implicitly apply migrations.

Use the dedicated migration command:

```text
cmd/migrate
```

Typical deployment:

```text
build
  ↓
run migrations
  ↓
start new application
```

---

# 18. Migration testing

CI must apply all migrations from zero against a clean PostgreSQL database.

The important invariant is:

> Current schema can always be reconstructed from repository migrations alone.

---

# 19. Schema ownership

Tables should have clear feature ownership.

Avoid a massive generic schema package.

Schema relationships may cross features when the domain requires it.

---

# 20. IDs

Prefer UUIDv7 for application entity IDs.

Generate application-owned IDs before inserts where useful.

Security tokens are separate random values and are not entity IDs.

---

# 21. Timestamps

Store timestamps using PostgreSQL timezone-aware timestamp types.

Application time should be treated as UTC.

Prefer database/application defaults consistently.

Avoid ambiguous local timestamps.

---

# 22. Pagination

For potentially large result sets, prefer cursor-based pagination.

Do not use unlimited list endpoints.

Enforce maximum page size.

Cursor implementation should be deterministic.

Example ordering:

```text
created_at DESC, id DESC
```

with both values represented in the cursor. `internal/example` shows the
pattern: the cursor is base64url of `created_at|id`, the query uses a row
comparison `(created_at, id) < ($2, $3)` backed by a matching index, and
one extra row is fetched to know whether a next page exists. Invalid
cursors answer `400 invalid_request`.

---

# 23. Account scoping

Account-owned data must be explicitly scoped.

Preferred:

```sql
SELECT ...
FROM resources
WHERE id = $1
  AND account_id = $2
```

This is safer than querying only by resource ID and relying entirely on later authorization.

Integration tests must verify cross-account isolation.

---

# 24. Test database behavior

Repository/integration tests use real PostgreSQL.

Do not substitute SQLite.

Tests should exercise:

* actual constraints
* transactions
* concurrent access where relevant
* migrations
* PostgreSQL-specific behavior

See `docs/testing.md`.

---

# 24a. Background jobs table

`platform/jobs` owns the `jobs` table. Workers claim runnable rows with

```sql
UPDATE jobs SET status = 'running', locked_at = now(), locked_by = $1, attempts = attempts + 1
WHERE id = (SELECT id FROM jobs WHERE ... ORDER BY run_at, id LIMIT 1 FOR UPDATE SKIP LOCKED)
```

so concurrent workers never execute the same job. A `running` job whose
`locked_at` is older than `JOBS_LOCK_TIMEOUT` is reclaimed (its worker is
assumed dead), which is why handlers must be idempotent. Failed jobs retry
with exponential backoff (2s, 4s, 8s, ... capped at 10 minutes) up to
`max_attempts`, then become `failed`; `jobs.Permanent(err)` skips retries.
Payloads are cleared when a job reaches `succeeded` or `failed`, so
short-lived secrets (verification links) do not linger; `last_error`,
`attempts` and timestamps remain for diagnosis.

---

# 25. Performance

Do not prematurely optimize queries.

However:

* avoid obvious N+1 queries
* add indexes required by actual query patterns
* use `EXPLAIN` when performance becomes relevant
* keep pagination bounded

Indexes should exist because a known access pattern needs them.

---

# 26. Database success criterion

A developer should be able to find and understand the persistence behavior for a feature without tracing through layers of generic infrastructure.

SQL should remain a visible part of the application.
