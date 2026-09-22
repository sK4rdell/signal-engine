package metrics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMetrics_CountsAndServesSnapshot(t *testing.T) {
	var m Metrics
	m.ObserveRequest(200, time.Millisecond)
	m.ObserveRequest(503, 2*time.Millisecond)
	m.ObserveJob(true)
	m.ObserveJob(false)
	m.RegisterGauge("db_pool_total_conns", func() int64 { return 3 })

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metricsz", nil))

	var got map[string]int64
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]int64{
		"http_requests_total":                2,
		"http_request_errors_total":          1,
		"http_request_duration_micros_total": 3000,
		"jobs_succeeded_total":               1,
		"jobs_failed_total":                  1,
		"db_pool_total_conns":                3,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %d, want %d", k, got[k], v)
		}
	}
}

func TestMetrics_NilIsSafe(t *testing.T) {
	var m *Metrics
	m.ObserveRequest(200, 0)
	m.ObserveJob(true)
	if len(m.Snapshot()) != 0 {
		t.Error("nil snapshot should be empty")
	}
}
