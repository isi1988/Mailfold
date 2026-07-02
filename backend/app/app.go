// Package app holds Mailfold's process-level bootstrap: it loads configuration,
// wires the mailcow client, authenticator and API server, runs the HTTP listener,
// and coordinates a graceful shutdown. It is exported (rather than living in
// package main) so that more than one entry point can share one bootstrap: the
// open-source binary in cmd/mailfold calls Run directly, and the enterprise binary
// blank-imports its PostgreSQL driver to register it with the storage package and
// then calls the same Run. Which databases exist is decided at link time; Run
// itself is database-agnostic and only ever talks to the storage registry.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/isi1988/Mailfold/backend/internal/api"
	"github.com/isi1988/Mailfold/backend/internal/auth"
	"github.com/isi1988/Mailfold/backend/internal/config"
	"github.com/isi1988/Mailfold/backend/internal/mailcow"
	"github.com/isi1988/Mailfold/backend/internal/ratelimit"
)

// Run boots the Mailfold backend and blocks until an interrupt or termination
// signal triggers a graceful shutdown. It returns a non-nil error only on a fatal
// startup or serving failure; a clean shutdown returns nil. Callers (the cmd
// entry points) are expected to log the error and exit non-zero.
func Run() error {
	// Emit structured JSON logs to stdout so that whatever runs the process (a
	// container runtime, systemd, and so on) can collect and parse them.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Configuration is loaded first because everything else depends on it.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Build the core dependencies from configuration: the client that talks to
	// mailcow, the authenticator that guards access, and the API server that
	// ties them together behind HTTP handlers.
	client := mailcow.NewClient(cfg.MailcowBaseURL, cfg.MailcowAPIKey, cfg.MailcowInsecureTLS)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, cfg.SessionTTL)
	loginLimiter := ratelimit.New(cfg.LoginRateMax, cfg.LoginRateWindow)
	server := api.NewServer(cfg, client, authn, loginLimiter, logger)

	// Periodically evict expired sessions, login-rate buckets, webmail sessions
	// and API-key rate state. The ticker is stopped on return to free it.
	gcTicker := time.NewTicker(10 * time.Minute)
	defer gcTicker.Stop()
	go func() {
		for range gcTicker.C {
			authn.GC()
			loginLimiter.GC()
			server.GCWebmail()
			server.GCAPIKeys()
		}
	}()

	// Configure the HTTP server. ReadHeaderTimeout bounds how long a client may
	// take to send request headers, protecting against slow-header clients that
	// would otherwise hold connections open.
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Serve in a goroutine so Run can wait for a shutdown signal. A serving
	// failure other than the expected ErrServerClosed is reported on serveErr.
	serveErr := make(chan error, 1)
	go func() {
		logger.Info("starting Mailfold backend", "addr", cfg.Addr, "mailcow", cfg.MailcowBaseURL)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	// Block until the server fails to start or the process receives an interrupt
	// (Ctrl+C) or termination signal. The channel is buffered so a signal that
	// arrives before this goroutine is ready is not missed.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-serveErr:
		return fmt.Errorf("http server: %w", err)
	case <-stop:
	}

	// Begin a graceful shutdown: stop accepting new connections and give
	// in-flight requests up to ten seconds to finish before forcing the server
	// closed. The context's cancel is deferred to avoid leaking it.
	logger.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return nil
}
