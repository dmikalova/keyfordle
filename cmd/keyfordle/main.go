// Command keyfordle serves KeyFordle, the daily KeyForge card-guessing game.
//
// It serves the React page and the four API routes the page calls. The day's
// card and the finished-game totals live in Postgres.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dmikalova/keyfordle/internal/cards"
	"github.com/dmikalova/keyfordle/internal/config"
	"github.com/dmikalova/keyfordle/internal/server"
	"github.com/dmikalova/keyfordle/internal/site"
	"github.com/dmikalova/keyfordle/internal/store"
)

const (
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 20 * time.Second
	startupTimeout    = 30 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("keyfordle stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	secretsPath := flag.String(
		"secrets",
		config.DefaultSecretsPath,
		"path to the mounted config secret",
	)
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	cfg, err := config.Load(*secretsPath)
	if err != nil {
		return fmt.Errorf("load the configuration: %w", err)
	}

	pool, err := cards.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()

	db, err := store.Open(startCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: server.New(server.Options{
			Pool:   pool,
			Store:  db,
			Site:   site.Handler(),
			Logger: log,
		}).Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	errs := make(chan error, 1)

	go func() {
		log.Info("listening", "port", cfg.Port, "cards", pool.Len())

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
		}
	}()

	select {
	case err := <-errs:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
		log.Info("shutting down")
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down: %w", err)
	}

	return nil
}
