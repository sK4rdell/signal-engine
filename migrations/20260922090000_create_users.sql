-- +goose Up
CREATE TABLE users (
    id                uuid        PRIMARY KEY,
    email             text        NOT NULL,
    email_normalized  text        NOT NULL,
    password_hash     text        NOT NULL,
    email_verified_at timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT users_email_normalized_key UNIQUE (email_normalized)
);

-- +goose Down
DROP TABLE users;
