# Go backend template

A small, production-capable Go backend for validating product ideas fast.
It solves the recurring backend concerns once (HTTP, validation, errors,
PostgreSQL, migrations, auth, accounts, email, background jobs, config,
health, graceful shutdown, security middleware, integration tests, Docker,
CI) so that a new product only needs feature packages.

Read [`CLAUDE.md`](CLAUDE.md) for the engineering rules and
[`docs/`](docs/) for the conventions this code follows.

## 1. Prerequisites

- Go 1.26+
- Docker with Compose (for PostgreSQL and Mailpit)
- `golangci-lint` (optional locally; CI runs it)

## 2. First startup

```bash
cp .env.example .env
make compose-up     # PostgreSQL 17 + Mailpit
make migrate        # apply migrations
make dev            # run the API (and, per .env, the in-process worker)
```

The API listens on `HTTP_ADDR` (default `:8080`). Try it:

```bash
curl -s localhost:8080/healthz
curl -s localhost:8080/readyz
```

Sign up, verify, log in, use an authenticated endpoint, log out, reset the
password and log in again with the new password. Every step below is also
covered by `TestAuth_DefinitionOfDoneFlow`.

```bash
# Unsafe (state-changing) requests must carry an allowed Origin; curl adds none, which is also accepted.
curl -s -c cookies -H 'Content-Type: application/json' -X POST localhost:8080/v1/auth/signup \
  -d '{"email":"me@example.com","password":"correct horse battery staple"}'

curl -s -b cookies localhost:8080/v1/auth/me
# -> open http://localhost:8025 (Mailpit), copy the token from the verification email
curl -s -H 'Content-Type: application/json' -X POST localhost:8080/v1/auth/verify-email -d '{"token":"<token>"}'

curl -s -b cookies -c cookies -X POST localhost:8080/v1/auth/logout
curl -s -c cookies -H 'Content-Type: application/json' -X POST localhost:8080/v1/auth/login \
  -d '{"email":"me@example.com","password":"correct horse battery staple"}'

curl -s -H 'Content-Type: application/json' -X POST localhost:8080/v1/auth/forgot-password -d '{"email":"me@example.com"}'
# -> copy the token from the reset email in Mailpit
curl -s -H 'Content-Type: application/json' -X POST localhost:8080/v1/auth/reset-password \
  -d '{"token":"<token>","password":"a brand new passphrase"}'
```

## 3. Development commands

```text
make help              list every target
make dev               run the API locally (sources .env)
make worker            run the background worker locally
make build             build bin/api, bin/worker, bin/migrate, bin/ingest
make ingest            ingest Arbetsmiljöverket inspection notices (from=, to=)
make ingest-klimatklivet  ingest Klimatklivet approved grants (from=, to=, force=1)
make test              run all tests against the compose PostgreSQL and Mailpit
make test-race         same, with the race detector
make lint              gofmt check, go vet, golangci-lint (when installed)
make fmt               gofmt
make vet               go vet
make compose-up/down   start/stop PostgreSQL and Mailpit
make reset-db          destroy the local database volume, recreate and migrate
make docker-build      build the production image
```

Configuration is read from environment variables only. `.env.example`
documents every variable; the Makefile sources `.env` when it exists, the
binaries never do. Production refuses insecure defaults (plain-text
database connections, non-HTTPS origins, non-secure cookies, text logs).

## 4. Migrations

Migrations are goose SQL files in [`migrations/`](migrations/), embedded
into the binaries and applied only by the dedicated command. The API never
migrates on startup.

```bash
make migrate                       # apply pending migrations
make migrate-down                  # roll back the latest one
make migration-status
make migration name=create_widgets # scaffold migrations/<timestamp>_create_widgets.sql
```

## 4b. Ingesting public events

Signal Engine turns public records into public events. Sources so far:
Arbetsmiljöverket's web diary (`docs/sources/arbetsmiljoverket.md`) and
Naturvårdsverket's Klimatklivet grant list (`docs/sources/klimatklivet.md`):

```bash
make ingest from=2026-09-20 to=2026-09-23        # Arbetsmiljöverket; inclusive dates, default yesterday
make ingest-klimatklivet from=2025-01-01         # Klimatklivet; decision-date window, default all
curl -s -b cookies 'localhost:8080/v1/public-events?source=klimatklivet&limit=100'
```

Re-running a window is idempotent. See `docs/public-events.md` for the
observation/event semantics and the API.

Never edit an applied migration; add a new one. CI applies every migration
to an empty database on each run, and `TestMigrator_UpDownUpFromEmptyDatabase`
checks that every `Down` section works.

## 5. Tests

```bash
make test
make test-race
```

Tests that need PostgreSQL read `TEST_DATABASE_URL` and are skipped when it
is unset. Each database-backed test gets its own database, copied from a
migrated template and dropped afterwards, so tests run in parallel without
sharing state. The SMTP test additionally needs `TEST_SMTP_ADDR` and
`TEST_MAILPIT_URL` (the compose Mailpit); the Makefile sets all three.

A feature test looks like this:

```go
app := testutil.NewApp(t)                 // real router, services, repositories, isolated DB
user := app.CreateUser()
client := app.AuthenticatedClient(user)   // real session cookie

res := client.PostJSON("/v1/accounts/"+user.AccountID.String()+"/examples", map[string]string{"title": "x"})
res.AssertStatus(t, http.StatusCreated)

app.RunJobs()                             // execute queued jobs
app.Email.Messages()                      // what the fake sender captured
```

See [`docs/testing.md`](docs/testing.md).

## 6. Local email testing

Compose runs [Mailpit](https://mailpit.axllent.org/): every email the
application sends lands there. Web UI: <http://localhost:8025>, SMTP:
`localhost:1025`. Verification and password-reset emails contain links to
`APP_BASE_URL` (your frontend) carrying the token as a query parameter; the
frontend posts that token to `/v1/auth/verify-email` or
`/v1/auth/reset-password`.

Emails are sent by background jobs. With `JOBS_WORKER_IN_API=true` (the
`.env.example` default) `make dev` runs the worker inside the API process;
otherwise run `make worker` in a second terminal.

## 7. Repository structure

```text
cmd/
  api/            HTTP server (optionally with embedded worker)
  worker/         background job worker
  migrate/        goose migrations: up | down | status | version
  ingest/         manual source ingestion: ingest arbetsmiljoverket|klimatklivet --from --to
internal/
  app/            composition root: wires config, infrastructure, features, routes, jobs
  auth/           users, sessions, signup/login/logout, email verification, password reset
  account/        accounts (workspaces) and memberships, membership middleware
  example/        example feature proving the conventions; delete it when starting a product
  publicevent/    source observations, canonical public events, GET /v1/public-events
  source/
    arbetsmiljoverket/  web diary client, parser and ingester (source-specific code only)
    klimatklivet/       dataset discovery, Excel parser and ingester (source-specific code only)
  platform/
    api/          typed handler wrapper, binding, strict JSON, validation, errors, middleware, router
    apperror/     application error type and stable public codes
    config/       typed configuration from environment variables
    database/     pgx pool, DBTX, InTx transaction helper, goose wiring
    email/        Sender interface, SMTP sender, log sender, recorder, embedded templates
    jobs/         PostgreSQL job queue: Enqueue and Worker
    logging/      slog construction and request-scoped logger
    metrics/      lightweight in-process counters served at /metricsz
    password/     Argon2id hashing
    ratelimit/    in-memory fixed-window limiter
    server/       http.Server construction, graceful shutdown, /healthz, /readyz
    testutil/     integration-test harness (App, factories, client); pgtest/ for bare databases
migrations/       goose SQL migrations (embedded)
docs/             architecture, API, database, testing and public-event conventions; docs/sources/ has source research
old/              previous implementation, reference only (see old/README.md)
```

### Endpoints

```text
GET    /healthz, /readyz, /metricsz

POST   /v1/auth/signup               POST /v1/auth/login
POST   /v1/auth/logout               POST /v1/auth/logout-all
GET    /v1/auth/me
POST   /v1/auth/verify-email         POST /v1/auth/resend-verification
POST   /v1/auth/forgot-password      POST /v1/auth/reset-password

GET    /v1/accounts                  GET  /v1/accounts/{accountID}

POST   /v1/accounts/{accountID}/examples
GET    /v1/accounts/{accountID}/examples?limit=&cursor=
GET    /v1/accounts/{accountID}/examples/{exampleID}
PATCH  /v1/accounts/{accountID}/examples/{exampleID}
DELETE /v1/accounts/{accountID}/examples/{exampleID}

GET    /v1/public-events?source=&event_type=&from=&to=&organisation_number=&limit=&cursor=
GET    /v1/public-events/{eventID}
```

### Renaming the project

The template's module path is `betemplate`. Rename it once when starting a
product:

```bash
make rename module=github.com/acme/widgets        # short name defaults to "widgets"
make rename module=github.com/acme/widgets name=widgets-api
```

The script rewrites `go.mod`, every import, the Docker image tag and the
email MIME boundary, then runs `go mod tidy`, `go build` and `go vet`. It
reads the current name from `go.mod`, so it can be run again later.

### Adding a feature

1. Create `internal/<feature>/` with `<feature>.model.go`,
   `<feature>.repository.go`, `handlers.go` (and a service file when there is
   real application logic).
2. Add a migration with `make migration name=...`.
3. Register routes in `internal/app/app.go` behind `authenticated` and, for
   account-scoped resources, `member`.
4. Write integration tests with `testutil.NewApp`.

`internal/example` is a complete worked example. Delete the package, its
migration and its registration line to start clean.

## Production

`Dockerfile` builds static binaries into a distroless, non-root image that
contains `api`, `worker` and `migrate`. Run `migrate up` before starting the
new version of `api` and `worker`. The image builds in CI.

Known limitation: rate limiting is in-memory per process, so the effective
limit scales with the number of API replicas. Move it to PostgreSQL or a
shared store if exact global limits matter.
