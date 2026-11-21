package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wescran/cranlab/internal/config"
	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/store"
	"github.com/wescran/cranlab/internal/web"
)

// siteHandler opens the disposable read model and attempts one complete backfill.
// An upstream outage is not a reason to stop serving the last good snapshot.
func siteHandler(ctx context.Context, cfg config.Config) (http.Handler, func(), error) {
	db, err := store.Open(cfg.CacheDBPath, cfg.DID, cfg.PublicationURI)
	if err != nil {
		return nil, nil, fmt.Errorf("open cache: %w", err)
	}
	client := pds.New(cfg.PDSURL, nil)
	if err := db.RefreshArticles(ctx, client); err != nil {
		slog.Warn("startup article refresh failed", "err", err)
	}
	h, err := web.New(cfg, web.Deps{Store: db, PDS: client})
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return h, func() { db.Close() }, nil
}

func run() error {
	logger := slog.Default()
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("server config load: %w", err)
	}
	if cfg.AppPassword == "" {
		logger.Info("No APP_PASSWORD configured, syndication disabled")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverHandler, closeStore, err := siteHandler(ctx, cfg)
	if err != nil {
		return fmt.Errorf("server: %w", err)
	}
	defer closeStore()
	server := &http.Server{
		Addr:              cfg.ListenAddr,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		Handler:           serverHandler,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		return fmt.Errorf("server: %w", err)
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown: %w", err)
		}
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		slog.Error("site stopped", "err", err)
		os.Exit(1)
	}
}
