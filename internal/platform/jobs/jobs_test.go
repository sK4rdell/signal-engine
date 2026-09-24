package jobs

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sK4rdell/signal-engine/internal/platform/config"
	"github.com/sK4rdell/signal-engine/internal/platform/database"
	"github.com/sK4rdell/signal-engine/internal/platform/logging"
	"github.com/sK4rdell/signal-engine/internal/platform/metrics"
	"github.com/sK4rdell/signal-engine/internal/platform/testutil/pgtest"
)

func testConfig() config.JobsConfig {
	return config.JobsConfig{
		PollInterval:    10 * time.Millisecond,
		Concurrency:     2,
		JobTimeout:      2 * time.Second,
		LockTimeout:     5 * time.Second,
		ShutdownTimeout: 2 * time.Second,
	}
}

func newTestWorker(t *testing.T, pool *pgxpool.Pool) *Worker {
	t.Helper()
	return NewWorker(pool, testConfig(), logging.Discard(), &metrics.Metrics{})
}

type payload struct {
	Name string `json:"name"`
}

func TestEnqueue_InsertsPendingJob(t *testing.T) {
	pool := pgtest.NewPool(t)
	ctx := context.Background()

	job, err := Enqueue(ctx, pool, "test.echo", payload{Name: "a"})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if job.Status != StatusPending || job.Attempts != 0 || job.MaxAttempts != DefaultMaxAttempts {
		t.Errorf("job = %+v", job)
	}
	var p payload
	if err := job.UnmarshalPayload(&p); err != nil || p.Name != "a" {
		t.Errorf("payload = %+v, %v", p, err)
	}

	later := time.Now().Add(time.Hour).UTC()
	job, err = Enqueue(ctx, pool, "test.echo", nil, RunAt(later), MaxAttempts(2))
	if err != nil {
		t.Fatal(err)
	}
	if !job.RunAt.Equal(later.Truncate(time.Microsecond)) || job.MaxAttempts != 2 {
		t.Errorf("job = %+v", job)
	}

	if _, err := Enqueue(ctx, pool, "", nil); err == nil {
		t.Error("empty type should be rejected")
	}
	if _, err := Enqueue(ctx, pool, "x", nil, MaxAttempts(0)); err == nil {
		t.Error("zero max attempts should be rejected")
	}
}

func TestEnqueue_InTransactionFollowsCommitAndRollback(t *testing.T) {
	pool := pgtest.NewPool(t)
	ctx := context.Background()

	err := database.InTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := Enqueue(ctx, tx, "test.rolled_back", nil); err != nil {
			return err
		}
		return errors.New("abort")
	})
	if err == nil {
		t.Fatal("expected callback error")
	}
	jobs, _ := ListByType(ctx, pool, "test.rolled_back")
	if len(jobs) != 0 {
		t.Errorf("rolled back job is visible: %+v", jobs)
	}

	err = database.InTx(ctx, pool, func(tx pgx.Tx) error {
		_, err := Enqueue(ctx, tx, "test.committed", nil)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	jobs, _ = ListByType(ctx, pool, "test.committed")
	if len(jobs) != 1 {
		t.Errorf("committed job missing: %+v", jobs)
	}
}

func TestWorker_RunOnceExecutesAndMarksSucceeded(t *testing.T) {
	pool := pgtest.NewPool(t)
	ctx := context.Background()
	w := newTestWorker(t, pool)

	var got []string
	w.Register("test.echo", func(ctx context.Context, job Job) error {
		var p payload
		if err := job.UnmarshalPayload(&p); err != nil {
			return err
		}
		got = append(got, p.Name)
		return nil
	})

	a, _ := Enqueue(ctx, pool, "test.echo", payload{Name: "a"})
	b, _ := Enqueue(ctx, pool, "test.echo", payload{Name: "b"})

	n, err := w.RunOnce(ctx)
	if err != nil || n != 2 {
		t.Fatalf("RunOnce = %d, %v", n, err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("executed = %v (oldest first expected)", got)
	}
	for _, id := range []Job{a, b} {
		job, _ := Get(ctx, pool, id.ID)
		if job.Status != StatusSucceeded || job.Attempts != 1 || job.CompletedAt == nil || job.LockedBy != nil {
			t.Errorf("job = %+v", job)
		}
		if job.Payload != nil {
			t.Errorf("payload should be cleared after completion, got %s", job.Payload)
		}
	}
	if w.metrics.Snapshot()["jobs_succeeded_total"] != 2 {
		t.Error("metrics not recorded")
	}
}

func TestWorker_RetriesWithBackoffThenFails(t *testing.T) {
	pool := pgtest.NewPool(t)
	ctx := context.Background()
	w := newTestWorker(t, pool)

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	w.now = func() time.Time { return now }

	calls := 0
	w.Register("test.flaky", func(ctx context.Context, job Job) error {
		calls++
		return errors.New("smtp unavailable")
	})
	job, _ := Enqueue(ctx, pool, "test.flaky", nil, MaxAttempts(3), RunAt(now))

	// Attempt 1 fails -> pending, run_at = now + Backoff(1).
	if n, _ := w.RunOnce(ctx); n != 1 {
		t.Fatalf("first run processed %d", n)
	}
	got, _ := Get(ctx, pool, job.ID)
	if got.Status != StatusPending || got.Attempts != 1 || got.LastError == nil || *got.LastError != "smtp unavailable" {
		t.Fatalf("after attempt 1: %+v", got)
	}
	if want := now.Add(Backoff(1)); !got.RunAt.Equal(want) {
		t.Errorf("run_at = %v, want %v", got.RunAt, want)
	}

	// Not yet due: nothing runs.
	if n, _ := w.RunOnce(ctx); n != 0 {
		t.Fatalf("job ran before its retry time")
	}

	now = now.Add(Backoff(1))
	if n, _ := w.RunOnce(ctx); n != 1 {
		t.Fatal("second attempt did not run")
	}
	now = now.Add(Backoff(2))
	if n, _ := w.RunOnce(ctx); n != 1 {
		t.Fatal("third attempt did not run")
	}
	got, _ = Get(ctx, pool, job.ID)
	if got.Status != StatusFailed || got.Attempts != 3 || got.CompletedAt == nil || got.Payload != nil {
		t.Errorf("after exhausting attempts: %+v", got)
	}
	if calls != 3 {
		t.Errorf("handler calls = %d", calls)
	}
	now = now.Add(time.Hour)
	if n, _ := w.RunOnce(ctx); n != 0 {
		t.Error("failed job must not run again")
	}
	if w.metrics.Snapshot()["jobs_failed_total"] != 3 {
		t.Error("failure metrics not recorded")
	}
}

func TestWorker_PermanentErrorSkipsRetries(t *testing.T) {
	pool := pgtest.NewPool(t)
	ctx := context.Background()
	w := newTestWorker(t, pool)
	w.Register("test.permanent", func(ctx context.Context, job Job) error {
		return Permanent(errors.New("bad payload"))
	})
	job, _ := Enqueue(ctx, pool, "test.permanent", nil)
	if _, err := w.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	got, _ := Get(ctx, pool, job.ID)
	if got.Status != StatusFailed || got.Attempts != 1 {
		t.Errorf("job = %+v", got)
	}
}

func TestWorker_UnknownJobTypeFailsClearly(t *testing.T) {
	pool := pgtest.NewPool(t)
	ctx := context.Background()
	w := newTestWorker(t, pool)

	job, _ := Enqueue(ctx, pool, "test.unknown", nil)
	if _, err := w.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	got, _ := Get(ctx, pool, job.ID)
	if got.Status != StatusFailed || got.LastError == nil || *got.LastError != `unknown job type "test.unknown"` {
		t.Errorf("job = %+v", got)
	}
}

func TestWorker_PanicIsRecoveredAndRetried(t *testing.T) {
	pool := pgtest.NewPool(t)
	ctx := context.Background()
	w := newTestWorker(t, pool)
	w.Register("test.panic", func(ctx context.Context, job Job) error {
		panic("boom")
	})
	job, _ := Enqueue(ctx, pool, "test.panic", nil)
	if _, err := w.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	got, _ := Get(ctx, pool, job.ID)
	if got.Status != StatusPending || got.LastError == nil || *got.LastError != "panic: boom" {
		t.Errorf("job = %+v", got)
	}
}

func TestWorker_FutureJobsWaitUntilDue(t *testing.T) {
	pool := pgtest.NewPool(t)
	ctx := context.Background()
	w := newTestWorker(t, pool)
	now := time.Now().UTC()
	w.now = func() time.Time { return now }
	w.Register("test.later", func(ctx context.Context, job Job) error { return nil })

	Enqueue(ctx, pool, "test.later", nil, RunAt(now.Add(time.Hour)))
	if n, _ := w.RunOnce(ctx); n != 0 {
		t.Fatal("future job ran early")
	}
	now = now.Add(2 * time.Hour)
	if n, _ := w.RunOnce(ctx); n != 1 {
		t.Fatal("due job did not run")
	}
}

func TestWorker_StaleLockIsReclaimedByAnotherWorker(t *testing.T) {
	pool := pgtest.NewPool(t)
	ctx := context.Background()
	now := time.Now().UTC()

	crashed := newTestWorker(t, pool)
	crashed.now = func() time.Time { return now }
	survivor := newTestWorker(t, pool)
	survivor.now = func() time.Time { return now }

	ran := 0
	survivor.Register("test.stale", func(ctx context.Context, job Job) error { ran++; return nil })

	job, _ := Enqueue(ctx, pool, "test.stale", nil, RunAt(now))
	// Simulate a worker that claimed the job and died: claim without finishing.
	if _, err := scanJob(pool.QueryRow(ctx, claimSQL, crashed.id, now, crashed.cfg.LockTimeout)); err != nil {
		t.Fatal(err)
	}

	if n, _ := survivor.RunOnce(ctx); n != 0 {
		t.Fatal("job with a fresh lock must not be reclaimed")
	}
	now = now.Add(crashed.cfg.LockTimeout + time.Second)
	if n, _ := survivor.RunOnce(ctx); n != 1 || ran != 1 {
		t.Fatalf("stale job not reclaimed: n=%d ran=%d", n, ran)
	}
	got, _ := Get(ctx, pool, job.ID)
	if got.Status != StatusSucceeded || got.Attempts != 2 {
		t.Errorf("job = %+v", got)
	}

	// The crashed worker coming back late cannot overwrite the outcome, and
	// must not count it either.
	before := crashed.metrics.Snapshot()
	for _, runErr := range []error{nil, errors.New("late failure"), Permanent(errors.New("late permanent"))} {
		if err := crashed.finish(ctx, got, runErr); !errors.Is(err, ErrLeaseLost) {
			t.Errorf("finish(%v) error = %v, want ErrLeaseLost", runErr, err)
		}
	}
	got, _ = Get(ctx, pool, job.ID)
	if got.Status != StatusSucceeded || got.LastError != nil {
		t.Errorf("stale worker overwrote the result: %+v", got)
	}
	if after := crashed.metrics.Snapshot(); after["jobs_succeeded_total"] != before["jobs_succeeded_total"] || after["jobs_failed_total"] != before["jobs_failed_total"] {
		t.Errorf("stale worker counted an outcome: before %v after %v", before, after)
	}
}

// TestWorker_StaleWorkerFinishingAfterReclaimDoesNotOverwrite runs the race
// for real: worker A is executing a job when its lock expires, worker B
// reclaims and completes the job, then A's handler returns. A must discard
// its outcome without touching B's state or counting a completion.
func TestWorker_StaleWorkerFinishingAfterReclaimDoesNotOverwrite(t *testing.T) {
	pool := pgtest.NewPool(t)
	ctx := context.Background()
	now := time.Now().UTC()

	slow := newTestWorker(t, pool)
	slow.now = func() time.Time { return now }
	fast := newTestWorker(t, pool)
	fast.now = func() time.Time { return now.Add(slow.cfg.LockTimeout + time.Second) }

	started := make(chan struct{})
	release := make(chan struct{})
	slow.Register("test.race", func(ctx context.Context, job Job) error {
		close(started)
		<-release
		return errors.New("slow worker failed")
	})
	fast.Register("test.race", func(ctx context.Context, job Job) error { return nil })

	job, _ := Enqueue(ctx, pool, "test.race", nil, RunAt(now))

	slowDone := make(chan error, 1)
	go func() {
		_, err := slow.RunOnce(ctx)
		slowDone <- err
	}()
	<-started

	// The lock is stale from fast's point of view: it reclaims and completes.
	if n, err := fast.RunOnce(ctx); err != nil || n != 1 {
		t.Fatalf("fast RunOnce = %d, %v", n, err)
	}
	got, _ := Get(ctx, pool, job.ID)
	if got.Status != StatusSucceeded {
		t.Fatalf("fast worker did not complete the job: %+v", got)
	}

	close(release)
	if err := <-slowDone; err != nil {
		t.Fatalf("slow RunOnce returned an error for a lost lease: %v", err)
	}

	got, _ = Get(ctx, pool, job.ID)
	if got.Status != StatusSucceeded || got.LastError != nil || got.Attempts != 2 {
		t.Errorf("slow worker overwrote the reclaimed job: %+v", got)
	}
	if s := slow.metrics.Snapshot(); s["jobs_failed_total"] != 0 || s["jobs_succeeded_total"] != 0 {
		t.Errorf("slow worker counted an outcome it did not persist: %v", s)
	}
	if s := fast.metrics.Snapshot(); s["jobs_succeeded_total"] != 1 {
		t.Errorf("fast worker metrics = %v", s)
	}
}

// TestWorker_ConcurrentWorkersNeverRunTheSameJob runs two workers with two
// goroutines each over slow jobs and checks that every job executed exactly
// once while several ran concurrently.
func TestWorker_ConcurrentWorkersNeverRunTheSameJob(t *testing.T) {
	pool := pgtest.NewPool(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const jobCount = 12
	var mu sync.Mutex
	executions := map[string]int{}
	inFlight, maxInFlight := 0, 0
	done := make(chan struct{}, jobCount)

	handler := func(ctx context.Context, job Job) error {
		mu.Lock()
		executions[job.ID.String()]++
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		mu.Unlock()

		time.Sleep(30 * time.Millisecond)

		mu.Lock()
		inFlight--
		mu.Unlock()
		done <- struct{}{}
		return nil
	}

	for i := 0; i < jobCount; i++ {
		if _, err := Enqueue(ctx, pool, "test.slow", nil); err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		w := newTestWorker(t, pool)
		w.Register("test.slow", handler)
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = w.Run(ctx)
		}()
	}

	deadline := time.After(10 * time.Second)
	for i := 0; i < jobCount; i++ {
		select {
		case <-done:
		case <-deadline:
			t.Fatalf("only %d of %d jobs completed in time", i, jobCount)
		}
	}
	cancel()
	wg.Wait()

	if len(executions) != jobCount {
		t.Errorf("executed %d distinct jobs, want %d", len(executions), jobCount)
	}
	for id, n := range executions {
		if n != 1 {
			t.Errorf("job %s executed %d times", id, n)
		}
	}
	if maxInFlight < 2 {
		t.Errorf("max concurrent executions = %d, expected parallelism", maxInFlight)
	}
	jobs, _ := ListByType(ctx, pool, "test.slow")
	for _, j := range jobs {
		if j.Status != StatusSucceeded {
			t.Errorf("job %s status %s", j.ID, j.Status)
		}
	}
}

func TestWorker_GracefulShutdownLetsInFlightJobFinish(t *testing.T) {
	pool := pgtest.NewPool(t)
	ctx, cancel := context.WithCancel(context.Background())

	w := newTestWorker(t, pool)
	started := make(chan struct{})
	release := make(chan struct{})
	finished := false
	w.Register("test.slow", func(ctx context.Context, job Job) error {
		close(started)
		<-release
		finished = true
		return nil
	})
	job, _ := Enqueue(ctx, pool, "test.slow", nil)

	runErr := make(chan error, 1)
	go func() { runErr <- w.Run(ctx) }()

	<-started
	cancel()
	select {
	case <-runErr:
		t.Fatal("Run returned while a job was in flight")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after in-flight job finished")
	}
	if !finished {
		t.Error("job did not finish")
	}
	got, _ := Get(context.Background(), pool, job.ID)
	if got.Status != StatusSucceeded {
		t.Errorf("job = %+v", got)
	}
}

func TestWorker_ShutdownTimeoutCancelsJobContext(t *testing.T) {
	pool := pgtest.NewPool(t)
	ctx, cancel := context.WithCancel(context.Background())

	cfg := testConfig()
	cfg.ShutdownTimeout = 100 * time.Millisecond
	w := NewWorker(pool, cfg, logging.Discard(), &metrics.Metrics{})
	started := make(chan struct{})
	var jobErr error
	w.Register("test.hang", func(ctx context.Context, job Job) error {
		close(started)
		<-ctx.Done()
		jobErr = ctx.Err()
		return ctx.Err()
	})
	Enqueue(ctx, pool, "test.hang", nil)

	runErr := make(chan error, 1)
	go func() { runErr <- w.Run(ctx) }()
	<-started
	cancel()
	select {
	case err := <-runErr:
		if err == nil {
			t.Error("expected shutdown timeout error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
	}
	if !errors.Is(jobErr, context.Canceled) {
		t.Errorf("job context error = %v", jobErr)
	}
}

// TestWorker_ShutdownIsBoundedWhenHandlerIgnoresCancellation covers a
// handler that never observes ctx.Done(): Run must still return within the
// shutdown timeout plus the cancel grace period.
func TestWorker_ShutdownIsBoundedWhenHandlerIgnoresCancellation(t *testing.T) {
	pool := pgtest.NewPool(t)
	ctx, cancel := context.WithCancel(context.Background())

	cfg := testConfig()
	cfg.ShutdownTimeout = 100 * time.Millisecond
	w := NewWorker(pool, cfg, logging.Discard(), &metrics.Metrics{})
	started := make(chan struct{})
	release := make(chan struct{})
	w.Register("test.stubborn", func(ctx context.Context, job Job) error {
		close(started)
		<-release // ignores ctx entirely
		return nil
	})
	t.Cleanup(func() { close(release) })
	Enqueue(ctx, pool, "test.stubborn", nil)

	runErr := make(chan error, 1)
	go func() { runErr <- w.Run(ctx) }()
	<-started
	cancel()

	select {
	case err := <-runErr:
		if err == nil {
			t.Error("expected a shutdown timeout error")
		}
	case <-time.After(cfg.ShutdownTimeout + cancelGracePeriod + 5*time.Second):
		t.Fatal("Run blocked on a handler that ignores cancellation")
	}
}

func TestWorker_RegisterTwicePanics(t *testing.T) {
	w := NewWorker(nil, testConfig(), logging.Discard(), nil)
	w.Register("x", func(context.Context, Job) error { return nil })
	defer func() {
		if recover() == nil {
			t.Error("expected panic")
		}
	}()
	w.Register("x", func(context.Context, Job) error { return nil })
}

func TestBackoff(t *testing.T) {
	if Backoff(1) != 2*time.Second || Backoff(3) != 8*time.Second {
		t.Errorf("Backoff(1)=%v Backoff(3)=%v", Backoff(1), Backoff(3))
	}
	if Backoff(30) != 10*time.Minute || Backoff(0) != 2*time.Second {
		t.Errorf("Backoff caps: %v %v", Backoff(30), Backoff(0))
	}
}
