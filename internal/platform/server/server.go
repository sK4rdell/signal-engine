// Package server owns the HTTP server lifecycle and the health endpoints.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"betemplate/internal/platform/config"
)

// New builds an http.Server with the configured timeouts.
func New(cfg config.HTTPConfig, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}

// Run serves until ctx is cancelled, then shuts down gracefully: the
// listener stops accepting connections and in-flight requests get up to
// shutdownTimeout to finish. Run returns nil after a clean shutdown.
func Run(ctx context.Context, srv *http.Server, shutdownTimeout time.Duration, logger *slog.Logger) error {
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("server: listen on %s: %w", srv.Addr, err)
	}
	return Serve(ctx, srv, ln, shutdownTimeout, logger)
}

// Serve is Run over an existing listener. Tests use it to bind port 0.
func Serve(ctx context.Context, srv *http.Server, ln net.Listener, shutdownTimeout time.Duration, logger *slog.Logger) error {
	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", ln.Addr().String())
		errCh <- srv.Serve(ln)
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("server: serve: %w", err)
	case <-ctx.Done():
	}

	logger.Info("http server shutting down", "timeout", shutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		// Requests still running after the timeout are cut off.
		_ = srv.Close()
		return fmt.Errorf("server: shutdown: %w", err)
	}
	<-errCh
	return nil
}
