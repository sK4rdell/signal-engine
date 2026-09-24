.DEFAULT_GOAL := help
SHELL := /bin/sh

# Local infrastructure defaults. Override in .env or on the command line.
POSTGRES_PORT     ?= 5432
DEV_DATABASE_URL  ?= postgres://app:app@localhost:$(POSTGRES_PORT)/app?sslmode=disable
TEST_DATABASE_URL ?= postgres://app:app@localhost:$(POSTGRES_PORT)/app_test?sslmode=disable
TEST_SMTP_ADDR    ?= localhost:1025
TEST_MAILPIT_URL  ?= http://localhost:8025

# The binaries read the environment only. Targets that run a binary source
# .env when it exists; explicit assignments on the command line still win.
LOAD_ENV := set -a; [ -f .env ] && . ./.env; set +a;

# Database for the migrate targets: `make DATABASE_URL=... migrate` (a Make
# variable) wins over .env, which wins over the compose default.
MIGRATE_DATABASE_URL = $(or $(DATABASE_URL),$${DATABASE_URL:-$(DEV_DATABASE_URL)})

.PHONY: help
help: ## Show available targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

.PHONY: dev
dev: ## Run the API locally (sources .env)
	$(LOAD_ENV) go run ./cmd/api

.PHONY: worker
worker: ## Run the background worker locally (sources .env)
	$(LOAD_ENV) go run ./cmd/worker

.PHONY: build
build: ## Build all binaries into bin/
	CGO_ENABLED=0 go build -trimpath -o bin/ ./cmd/...

.PHONY: test
test: ## Run all tests against the compose PostgreSQL and Mailpit
	$(LOAD_ENV) TEST_DATABASE_URL="$${TEST_DATABASE_URL:-$(TEST_DATABASE_URL)}" \
		TEST_SMTP_ADDR="$${TEST_SMTP_ADDR:-$(TEST_SMTP_ADDR)}" TEST_MAILPIT_URL="$${TEST_MAILPIT_URL:-$(TEST_MAILPIT_URL)}" \
		go test -count=1 ./...

.PHONY: test-race
test-race: ## Run all tests with the race detector
	$(LOAD_ENV) TEST_DATABASE_URL="$${TEST_DATABASE_URL:-$(TEST_DATABASE_URL)}" \
		TEST_SMTP_ADDR="$${TEST_SMTP_ADDR:-$(TEST_SMTP_ADDR)}" TEST_MAILPIT_URL="$${TEST_MAILPIT_URL:-$(TEST_MAILPIT_URL)}" \
		go test -race -count=1 ./...

.PHONY: fmt
fmt: ## Format all Go files
	gofmt -w $$(find . -name '*.go' -not -path './old/*')

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: ## Check formatting, vet and run golangci-lint when installed
	@unformatted=$$(gofmt -l $$(find . -name '*.go' -not -path './old/*')); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed:"; echo "$$unformatted"; exit 1; \
	fi
	go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed; ran gofmt + go vet only."; \
		echo "Install: https://golangci-lint.run/welcome/install/"; \
	fi

.PHONY: compose-up
compose-up: ## Start PostgreSQL and Mailpit
	docker compose up -d --wait

.PHONY: compose-down
compose-down: ## Stop PostgreSQL and Mailpit
	docker compose down

.PHONY: reset-db
reset-db: ## Destroy the local database volume, recreate it and migrate
	docker compose down -v
	$(MAKE) compose-up
	$(MAKE) migrate

.PHONY: migrate
migrate: ## Apply all pending migrations
	$(LOAD_ENV) DATABASE_URL="$(MIGRATE_DATABASE_URL)" go run ./cmd/migrate up

.PHONY: migrate-down
migrate-down: ## Roll back the most recent migration
	$(LOAD_ENV) DATABASE_URL="$(MIGRATE_DATABASE_URL)" go run ./cmd/migrate down

.PHONY: migration-status
migration-status: ## Show migration status
	$(LOAD_ENV) DATABASE_URL="$(MIGRATE_DATABASE_URL)" go run ./cmd/migrate status

.PHONY: migration
migration: ## Scaffold a migration: make migration name=create_foo
	@test -n "$(name)" || { echo "usage: make migration name=create_foo"; exit 1; }
	@file="migrations/$$(date -u +%Y%m%d%H%M%S)_$(name).sql"; \
	printf -- '-- +goose Up\n\n\n-- +goose Down\n\n' > "$$file"; \
	echo "created $$file"

.PHONY: rename
rename: ## Rename the project: make rename module=github.com/acme/widgets [name=widgets]
	@test -n "$(module)" || { echo "usage: make rename module=github.com/acme/widgets [name=widgets]"; exit 1; }
	scripts/rename.sh "$(module)" $(name)

.PHONY: docker-build
docker-build: ## Build the production image
	docker build -t signal-engine:local .

.PHONY: clean
clean: ## Remove build output
	rm -rf bin/
