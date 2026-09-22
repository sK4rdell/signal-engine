// Package jobs is a small PostgreSQL-backed background job queue.
//
// Jobs are rows in the jobs table. Producers enqueue them on the pool or,
// preferably, inside the transaction that makes the change they follow
// from, so the job exists exactly when the change is committed. Workers
// claim runnable jobs with FOR UPDATE SKIP LOCKED, so any number of worker
// processes can run against the same table without executing a job twice.
//
// Failed jobs are retried with exponential backoff up to max_attempts and
// then marked failed. A job whose worker died mid-execution is reclaimed
// once its lock is older than the configured lock timeout, so handlers must
// be idempotent. Payloads are cleared once a job reaches a terminal state so
// short-lived secrets (verification links) do not linger in the table.
package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"betemplate/internal/platform/database"
)

// Status of a job.
type Status string

// Job statuses.
const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

// DefaultMaxAttempts applies when Enqueue is not given MaxAttempts.
const DefaultMaxAttempts = 5

// Job is one row of the jobs table.
type Job struct {
	ID          uuid.UUID
	Type        string
	Payload     json.RawMessage
	Status      Status
	Attempts    int
	MaxAttempts int
	RunAt       time.Time
	LockedAt    *time.Time
	LockedBy    *string
	LastError   *string
	CreatedAt   time.Time
	CompletedAt *time.Time
}

// UnmarshalPayload decodes the payload into v.
func (j Job) UnmarshalPayload(v any) error {
	if err := json.Unmarshal(j.Payload, v); err != nil {
		return fmt.Errorf("jobs: decode payload of %s job %s: %w", j.Type, j.ID, err)
	}
	return nil
}

// EnqueueOption customises Enqueue.
type EnqueueOption func(*enqueueOptions)

type enqueueOptions struct {
	runAt       *time.Time
	maxAttempts int
}

// RunAt schedules the job for a future time instead of immediately.
func RunAt(t time.Time) EnqueueOption {
	return func(o *enqueueOptions) { o.runAt = &t }
}

// MaxAttempts overrides DefaultMaxAttempts.
func MaxAttempts(n int) EnqueueOption {
	return func(o *enqueueOptions) { o.maxAttempts = n }
}

const enqueueSQL = `
	INSERT INTO jobs (id, type, payload, status, max_attempts, run_at)
	VALUES ($1, $2, $3, 'pending', $4, $5)
	RETURNING ` + jobColumns

// Enqueue inserts a pending job. Pass a transaction as db to make the job
// part of the surrounding operation.
func Enqueue(ctx context.Context, db database.DBTX, jobType string, payload any, opts ...EnqueueOption) (Job, error) {
	o := enqueueOptions{maxAttempts: DefaultMaxAttempts}
	for _, opt := range opts {
		opt(&o)
	}
	if jobType == "" {
		return Job{}, fmt.Errorf("jobs: job type is required")
	}
	if o.maxAttempts < 1 {
		return Job{}, fmt.Errorf("jobs: max attempts must be at least 1")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return Job{}, fmt.Errorf("jobs: encode payload: %w", err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Job{}, fmt.Errorf("jobs: generate id: %w", err)
	}
	runAt := time.Now().UTC()
	if o.runAt != nil {
		runAt = o.runAt.UTC()
	}
	job, err := scanJob(db.QueryRow(ctx, enqueueSQL, id, jobType, raw, o.maxAttempts, runAt))
	if err != nil {
		return Job{}, fmt.Errorf("jobs: enqueue %s: %w", jobType, err)
	}
	return job, nil
}

const jobColumns = `id, type, payload, status, attempts, max_attempts, run_at, locked_at, locked_by, last_error, created_at, completed_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (Job, error) {
	var j Job
	err := row.Scan(&j.ID, &j.Type, &j.Payload, &j.Status, &j.Attempts, &j.MaxAttempts, &j.RunAt,
		&j.LockedAt, &j.LockedBy, &j.LastError, &j.CreatedAt, &j.CompletedAt)
	return j, err
}

const getJobSQL = `SELECT ` + jobColumns + ` FROM jobs WHERE id = $1`

// Get returns one job by ID. Tests and admin tooling use it.
func Get(ctx context.Context, db database.DBTX, id uuid.UUID) (Job, error) {
	job, err := scanJob(db.QueryRow(ctx, getJobSQL, id))
	if err != nil {
		return Job{}, fmt.Errorf("jobs: get %s: %w", id, err)
	}
	return job, nil
}

const listByTypeSQL = `SELECT ` + jobColumns + ` FROM jobs WHERE type = $1 ORDER BY created_at, id`

// ListByType returns every job of a type, oldest first. Tests use it to
// assert on enqueued work.
func ListByType(ctx context.Context, db database.DBTX, jobType string) ([]Job, error) {
	rows, err := db.Query(ctx, listByTypeSQL, jobType)
	if err != nil {
		return nil, fmt.Errorf("jobs: list %s: %w", jobType, err)
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("jobs: scan: %w", err)
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("jobs: read rows: %w", err)
	}
	return out, nil
}
