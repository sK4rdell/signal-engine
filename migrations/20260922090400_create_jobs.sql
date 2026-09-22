-- +goose Up
CREATE TABLE jobs (
    id           uuid        PRIMARY KEY,
    type         text        NOT NULL,
    payload      jsonb,
    status       text        NOT NULL DEFAULT 'pending',
    attempts     integer     NOT NULL DEFAULT 0,
    max_attempts integer     NOT NULL,
    run_at       timestamptz NOT NULL DEFAULT now(),
    locked_at    timestamptz,
    locked_by    text,
    last_error   text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,

    CONSTRAINT jobs_status_check CHECK (status IN ('pending', 'running', 'succeeded', 'failed')),
    CONSTRAINT jobs_max_attempts_check CHECK (max_attempts >= 1)
);

-- The claim query scans runnable jobs ordered by run_at.
CREATE INDEX jobs_runnable_idx ON jobs (run_at) WHERE status IN ('pending', 'running');

-- +goose Down
DROP TABLE jobs;
