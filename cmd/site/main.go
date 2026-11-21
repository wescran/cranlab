package main

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

	"github.com/wescran/cranlab/internal/config"
	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/store"
	"github.com/wescran/cranlab/internal/syndicate"
	"github.com/wescran/cranlab/internal/web"
	"github.com/wescran/cranlab/internal/webmention"
)

// siteHandler opens the disposable read model and attempts one complete backfill.
// An upstream outage is not a reason to stop serving the last good snapshot.
func siteHandler(ctx context.Context, cfg config.Config, interactions web.InteractionReader) (http.Handler, func(), error) {
	db, err := store.Open(cfg.CacheDBPath, cfg.DID, cfg.PublicationURI, cfg.SiteURL)
	if err != nil {
		return nil, nil, fmt.Errorf("open cache: %w", err)
	}
	client := pds.New(cfg.PDSURL, nil)
	h, err := web.New(cfg, web.Deps{Store: db, Blobs: client, Interactions: interactions})
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	refresh := func(ctx context.Context, full bool) error {
		articleErr := db.RefreshArticles(ctx, client)
		postErr := db.RefreshPosts(ctx, client, full)
		return errors.Join(articleErr, postErr)
	}
	if cfg.AppPassword != "" {
		runner := syndicate.New(db, client, cfg.DID, cfg.PublicationURI, cfg.SiteURL, cfg.AppPassword, slog.Default(), time.Now)
		refresh = afterRefresh(refresh, runner.Run)
	}
	stop := store.StartSync(ctx, client.SubscribeRepos, refresh)
	return h, func() { stop(); db.Close() }, nil
}

// Never repair from a stale or partially refreshed snapshot. The existing
// sync loop owns scheduling, serialization, cancellation and shutdown.
func afterRefresh(refresh func(context.Context, bool) error, repair func(context.Context)) func(context.Context, bool) error {
	return func(ctx context.Context, full bool) error {
		if err := refresh(ctx, full); err != nil {
			return err
		}
		repair(ctx)
		return nil
	}
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
	serverHandler, stopSite, err := siteHandler(ctx, cfg, webmention.New("https://webmention.io", http.DefaultClient))
	if err != nil {
		return fmt.Errorf("server: %w", err)
	}
	defer stopSite()
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
