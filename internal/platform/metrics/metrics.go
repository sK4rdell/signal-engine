// Package metrics keeps lightweight in-process counters.
//
// It is deliberately tiny: a handful of atomic counters exposed as JSON on
// an internal endpoint, enough to see request, error and job rates without
// booting an observability stack. Swap the implementation for Prometheus or
// OpenTelemetry when a product needs it.
package metrics

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds the process counters. The zero value is ready to use.
type Metrics struct {
	requestsTotal         atomic.Int64
	requestErrorsTotal    atomic.Int64
	requestDurationMicros atomic.Int64
	jobsSucceededTotal    atomic.Int64
	jobsFailedTotal       atomic.Int64

	mu     sync.Mutex
	gauges map[string]func() int64
}

// ObserveRequest records one finished HTTP request.
func (m *Metrics) ObserveRequest(status int, duration time.Duration) {
	if m == nil {
		return
	}
	m.requestsTotal.Add(1)
	if status >= 500 {
		m.requestErrorsTotal.Add(1)
	}
	m.requestDurationMicros.Add(duration.Microseconds())
}

// ObserveJob records one finished job execution.
func (m *Metrics) ObserveJob(succeeded bool) {
	if m == nil {
		return
	}
	if succeeded {
		m.jobsSucceededTotal.Add(1)
	} else {
		m.jobsFailedTotal.Add(1)
	}
}

// RegisterGauge adds a value read on every snapshot, such as pool statistics.
func (m *Metrics) RegisterGauge(name string, read func() int64) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gauges == nil {
		m.gauges = map[string]func() int64{}
	}
	m.gauges[name] = read
}

// Snapshot returns the current values.
func (m *Metrics) Snapshot() map[string]int64 {
	out := map[string]int64{}
	if m == nil {
		return out
	}
	out["http_requests_total"] = m.requestsTotal.Load()
	out["http_request_errors_total"] = m.requestErrorsTotal.Load()
	out["http_request_duration_micros_total"] = m.requestDurationMicros.Load()
	out["jobs_succeeded_total"] = m.jobsSucceededTotal.Load()
	out["jobs_failed_total"] = m.jobsFailedTotal.Load()
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, read := range m.gauges {
		out[name] = read()
	}
	return out
}

// Handler serves the snapshot as JSON.
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(m.Snapshot())
	})
}
