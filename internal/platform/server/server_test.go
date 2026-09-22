package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"betemplate/internal/platform/config"
	"betemplate/internal/platform/logging"
)

type pinger struct{ err error }

func (p pinger) Ping(context.Context) error { return p.err }

func TestHealthz_DoesNotDependOnDatabase(t *testing.T) {
	rec := httptest.NewRecorder()
	Healthz().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestReadyz_ReflectsDatabase(t *testing.T) {
	rec := httptest.NewRecorder()
	Readyz(pinger{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("healthy status = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	Readyz(pinger{err: errors.New("down")}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unhealthy status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"database":"failed"`) {
		t.Errorf("body = %s", rec.Body.String())
	}
}

// TestServe_GracefulShutdownLetsInFlightRequestsFinish starts a real server,
// begins a slow request, cancels the run context and verifies that the slow
// request still completes while new connections are refused.
func TestServe_GracefulShutdownLetsInFlightRequestsFinish(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "done")
	})

	srv := New(config.HTTPConfig{ReadHeaderTimeout: time.Second, IdleTimeout: time.Second}, handler)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- Serve(ctx, srv, ln, 5*time.Second, logging.Discard()) }()

	var wg sync.WaitGroup
	wg.Add(1)
	var body string
	var reqErr error
	go func() {
		defer wg.Done()
		res, err := http.Get("http://" + ln.Addr().String() + "/slow")
		if err != nil {
			reqErr = err
			return
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		body = string(b)
	}()

	<-started
	cancel()

	// Shutdown has begun; the listener must be closed soon.
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", ln.Addr().String(), 200*time.Millisecond)
		if err != nil {
			break
		}
		_ = conn.Close()
		if time.Now().After(deadline) {
			t.Fatal("listener still accepting connections after shutdown started")
		}
		time.Sleep(20 * time.Millisecond)
	}

	close(release)
	wg.Wait()
	if reqErr != nil {
		t.Fatalf("in-flight request failed: %v", reqErr)
	}
	if body != "done" {
		t.Errorf("body = %q", body)
	}

	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("Serve returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after shutdown")
	}
}

func TestServe_ReturnsWhenShutdownTimeoutExpires(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
	})
	srv := New(config.HTTPConfig{ReadHeaderTimeout: time.Second}, handler)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- Serve(ctx, srv, ln, 100*time.Millisecond, logging.Discard()) }()

	go func() {
		res, err := http.Get("http://" + ln.Addr().String() + "/")
		if err == nil {
			res.Body.Close()
		}
	}()
	<-started
	cancel()

	select {
	case err := <-runErr:
		if err == nil {
			t.Fatal("expected a shutdown timeout error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after the shutdown timeout")
	}
	close(release)
}
