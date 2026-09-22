-- +goose Up
-- Example feature (internal/example). Delete this migration together with
-- the package when starting a real product.
CREATE TABLE examples (
    id         uuid        PRIMARY KEY,
    account_id uuid        NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    title      text        NOT NULL,
    note       text        NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Keyset pagination per account: ORDER BY created_at DESC, id DESC.
CREATE INDEX examples_account_created_idx ON examples (account_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE examples;
