-- +goose Up
CREATE TABLE accounts (
    id         uuid        PRIMARY KEY,
    name       text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE account_memberships (
    account_id uuid        NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (account_id, user_id),
    CONSTRAINT account_memberships_role_check CHECK (role IN ('owner', 'member'))
);

CREATE INDEX account_memberships_user_id_idx ON account_memberships (user_id);

-- +goose Down
DROP TABLE account_memberships;
DROP TABLE accounts;
