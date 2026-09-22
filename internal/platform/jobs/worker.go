package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"betemplate/internal/platform/config"
	"betemplate/internal/platform/logging"
	"betemplate/internal/platform/metrics"
)

// Handler executes one job. Returning an error schedules a retry (or marks
// the job failed after the last attempt). Handlers must be idempotent: a job
// can run again after a crash or timeout.
type Handler func(ctx context.Context, job Job) error

// Worker claims and executes jobs.
type Worker struct {
	pool     *pgxpool.Pool
	cfg      config.JobsConfig
	logger   *slog.Logger
	metrics  *metrics.Metrics
	id       string
	handlers map[string]Handler

	// now and backoff are overridable for tests.
	now     func() time.Time
	backoff func(attempt int) time.Duration
}

// NewWorker builds a worker. Register handlers before calling Run.
func NewWorker(pool *pgxpool.Pool, cfg config.JobsConfig, logger *slog.Logger, m *metrics.Metrics) *Worker {
	host, _ := os.Hostname()
	return &Worker{
		pool:     pool,
		cfg:      cfg,
		logger:   logger,
		metrics:  m,
		id:       fmt.Sprintf("%s-%d-%s", host, os.Getpid(), uuid.NewString()[:8]),
		handlers: map[string]Handler{},
		now:      time.Now,
		backoff:  Backoff,
	}
}

// Register binds a handler to a job type. Registering a type twice is a
// programming error and panics at startup.
func (w *Worker) Register(jobType string, h Handler) {
	if _, exists := w.handlers[jobType]; exists {
		panic(fmt.Sprintf("jobs: handler for %q registered twice", jobType))
	}
	w.handlers[jobType] = h
}

// Backoff is the default retry delay: 2^attempt seconds, capped at ten
// minutes (2s, 4s, 8s, 16s, ...).
func Backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 20 {
		attempt = 20
	}
	d := time.Duration(1<<uint(attempt)) * time.Second
	if d > 10*time.Minute {
		d = 10 * time.Minute
	}
	return d
}

// Run executes jobs with cfg.Concurrency goroutines until ctx is cancelled.
// It then stops claiming, gives in-flight jobs cfg.ShutdownTimeout to
// finish, and returns.
func (w *Worker) Run(ctx context.Context) error {
	// Handlers get a context that outlives ctx by the shutdown grace period,
	// so a job in progress is not cut off the instant shutdown starts.
	jobCtx, cancelJobs := context.WithCancel(context.Background())
	defer cancelJobs()

	w.logger.Info("worker started", "worker_id", w.id, "concurrency", w.cfg.Concurrency, "types", w.types())

	var wg sync.WaitGroup
	for i := 0; i < w.cfg.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.loop(ctx, jobCtx)
		}()
	}

	<-ctx.Done()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		w.logger.Info("worker stopped", "worker_id", w.id)
		return nil
	case <-time.After(w.cfg.ShutdownTimeout):
		cancelJobs()
		<-done
		w.logger.Warn("worker stopped after shutdown timeout; in-flight jobs were cancelled", "worker_id", w.id)
		return errors.New("jobs: shutdown timeout exceeded")
	}
}

func (w *Worker) loop(ctx, jobCtx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		processed, err := w.processOne(ctx, jobCtx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.logger.Error("worker iteration failed", "worker_id", w.id, "error", err)
		}
		if processed && err == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(w.cfg.PollInterval):
		}
	}
}

// RunOnce processes runnable jobs until none is left and returns how many
// it executed. Tests and one-shot tooling use it.
func (w *Worker) RunOnce(ctx context.Context) (int, error) {
	n := 0
	for {
		processed, err := w.processOne(ctx, ctx)
		if err != nil {
			return n, err
		}
		if !processed {
			return n, nil
		}
		n++
	}
}

const claimSQL = `
	UPDATE jobs
	SET status = 'running', locked_at = $2, locked_by = $1, attempts = attempts + 1
	WHERE id = (
		SELECT id
		FROM jobs
		WHERE (status = 'pending' AND run_at <= $2)
		   OR (status = 'running' AND locked_at < $2 - $3::interval)
		ORDER BY run_at, id
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	)
	RETURNING ` + jobColumns

// processOne claims and runs a single job. It reports false when nothing
// was runnable.
func (w *Worker) processOne(claimCtx, jobCtx context.Context) (bool, error) {
	job, err := scanJob(w.pool.QueryRow(claimCtx, claimSQL, w.id, w.now().UTC(), w.cfg.LockTimeout))
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("jobs: claim: %w", err)
	}

	runErr := w.execute(jobCtx, job)

	// Recording the outcome must not depend on a context that shutdown may
	// already have cancelled; otherwise a finished job would stay "running"
	// until its lock times out and it would run again.
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(jobCtx), 10*time.Second)
	defer cancel()
	if err := w.finish(finishCtx, job, runErr); err != nil {
		if errors.Is(err, ErrLeaseLost) {
			// Not a worker failure: the job continued under a new owner. The
			// handler ran twice, which is why handlers must be idempotent.
			w.logger.Warn("job outcome discarded", "job_id", job.ID, "job_type", job.Type, "attempt", job.Attempts, "worker_id", w.id, "error", err)
			return true, nil
		}
		return true, err
	}
	return true, nil
}

func (w *Worker) execute(parent context.Context, job Job) (err error) {
	handler, ok := w.handlers[job.Type]
	if !ok {
		return &permanentError{err: fmt.Errorf("unknown job type %q", job.Type)}
	}

	ctx, cancel := context.WithTimeout(parent, w.cfg.JobTimeout)
	defer cancel()
	ctx = logging.WithLogger(ctx, w.logger)
	logging.Add(ctx, "job_id", job.ID, "job_type", job.Type, "attempt", job.Attempts, "worker_id", w.id)

	defer func() {
		if rec := recover(); rec != nil {
			logging.FromContext(ctx).Error("job panicked", "panic", fmt.Sprint(rec), "stack", string(debug.Stack()))
			err = fmt.Errorf("panic: %v", rec)
		}
	}()
	return handler(ctx, job)
}

// ErrLeaseLost is returned by finish when the job was reclaimed by another
// worker (after a lock timeout) before this worker could record its
// outcome. The other worker's state is left untouched.
var ErrLeaseLost = errors.New("jobs: lease lost, job was reclaimed by another worker")

// permanentError marks a failure that retrying cannot fix.
type permanentError struct{ err error }

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

// Permanent wraps err so the job is marked failed without further retries.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &permanentError{err: err}
}

const succeedSQL = `
	UPDATE jobs
	SET status = 'succeeded', completed_at = $3, locked_at = NULL, locked_by = NULL, payload = NULL, last_error = NULL
	WHERE id = $1 AND locked_by = $2
`

const failSQL = `
	UPDATE jobs
	SET status = 'failed', completed_at = $3, locked_at = NULL, locked_by = NULL, payload = NULL, last_error = $4
	WHERE id = $1 AND locked_by = $2
`

const retrySQL = `
	UPDATE jobs
	SET status = 'pending', run_at = $3, locked_at = NULL, locked_by = NULL, last_error = $4
	WHERE id = $1 AND locked_by = $2
`

// finish records the outcome. Every statement is guarded by locked_by, and
// the row count is checked: a job reclaimed by another worker after a lock
// timeout is never overwritten, and its outcome is neither logged as
// recorded nor counted in metrics. Metrics and outcome logs happen only
// after the state transition succeeded.
func (w *Worker) finish(ctx context.Context, job Job, runErr error) error {
	now := w.now().UTC()
	logger := w.logger.With("job_id", job.ID, "job_type", job.Type, "attempt", job.Attempts, "worker_id", w.id)

	if runErr == nil {
		if err := w.transition(ctx, "mark succeeded", succeedSQL, job.ID, w.id, now); err != nil {
			return err
		}
		w.metrics.ObserveJob(true)
		logger.Info("job succeeded")
		return nil
	}

	var permanent *permanentError
	exhausted := job.Attempts >= job.MaxAttempts
	if errors.As(runErr, &permanent) || exhausted {
		if err := w.transition(ctx, "mark failed", failSQL, job.ID, w.id, now, runErr.Error()); err != nil {
			return err
		}
		w.metrics.ObserveJob(false)
		logger.Error("job failed permanently", "error", runErr, "exhausted", exhausted)
		return nil
	}

	delay := w.backoff(job.Attempts)
	if err := w.transition(ctx, "schedule retry", retrySQL, job.ID, w.id, now.Add(delay), runErr.Error()); err != nil {
		return err
	}
	w.metrics.ObserveJob(false)
	logger.Warn("job failed, will retry", "error", runErr, "retry_in", delay)
	return nil
}

// transition runs one outcome statement and verifies this worker still
// owned the job: exactly one row must change.
func (w *Worker) transition(ctx context.Context, what, sql string, args ...any) error {
	tag, err := w.pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("jobs: %s: %w", what, err)
	}
	if tag.RowsAffected() != 1 {
		return ErrLeaseLost
	}
	return nil
}

func (w *Worker) types() []string {
	out := make([]string, 0, len(w.handlers))
	for t := range w.handlers {
		out = append(out, t)
	}
	return out
}
