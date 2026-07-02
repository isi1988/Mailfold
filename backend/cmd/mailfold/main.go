// Command mailfold is the entry point for the Mailfold backend HTTP server. It
// wires together the application's components — configuration, the mailcow API
// client, the authenticator, and the API server — starts the HTTP listener, and
// coordinates a graceful shutdown when the process is asked to stop. Keeping all
// of this process-level orchestration in main leaves the individual packages
// free of startup and signal-handling concerns.
package main

import (
	"context"
	"errors"
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

// main boots the Mailfold backend: it loads configuration, constructs the
// server and its dependencies, runs the HTTP listener in the background, and
// blocks until an interrupt or termination signal triggers a graceful shutdown.
func main() {
	// Emit structured JSON logs to stdout so that whatever runs the process (a
	// container runtime, systemd, and so on) can collect and parse them.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Configuration is loaded first because everything else depends on it; a
	// configuration error is unrecoverable, so log it and exit non-zero.
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Build the core dependencies from configuration: the client that talks to
	// mailcow, the authenticator that guards access, and the API server that
	// ties them together behind HTTP handlers.
	client := mailcow.NewClient(cfg.MailcowBaseURL, cfg.MailcowAPIKey, cfg.MailcowInsecureTLS)
	authn := auth.New(cfg.AdminUser, cfg.AdminPassword, cfg.SessionTTL)
	loginLimiter := ratelimit.New(cfg.LoginRateMax, cfg.LoginRateWindow)
	server := api.NewServer(cfg, client, authn, loginLimiter, logger)

	// Periodically evict expired sessions. Validate only removes an expired
	// token if it is presented again, so this background sweep reclaims the
	// memory of sessions that are simply abandoned. The ticker is stopped on
	// return to release its resources.
	gcTicker := time.NewTicker(10 * time.Minute)
	defer gcTicker.Stop()
	go func() {
		for range gcTicker.C {
			authn.GC()
			loginLimiter.GC()
		}
	}()

	// Configure the HTTP server. ReadHeaderTimeout bounds how long a client may
	// take to send request headers, which protects against slow-header
	// denial-of-service clients that would otherwise hold connections open.
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Serve in a separate goroutine so that main can go on to wait for a
	// shutdown signal. ListenAndServe always returns a non-nil error; the
	// expected http.ErrServerClosed (returned when Shutdown is called) is
	// ignored, while any other error is fatal.
	go func() {
		logger.Info("starting Mailfold backend", "addr", cfg.Addr, "mailcow", cfg.MailcowBaseURL)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server error", "error", err)
			os.Exit(1)
		}
	}()

	// Block until the process receives an interrupt (Ctrl+C) or termination
	// signal. The channel is buffered so the signal is not missed if it arrives
	// before this goroutine is ready to receive it.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	// Begin a graceful shutdown: stop accepting new connections and give
	// in-flight requests up to ten seconds to finish before forcing the server
	// closed. The context's cancel is deferred to avoid leaking it.
	logger.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
