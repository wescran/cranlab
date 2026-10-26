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
)

func run() error {
	logger := slog.Default()
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("server config load: %w", err)
	}
	if cfg.AppPassword == "" {
		logger.Info("No APP_PASSWORD configured, syndication disabled")
	}
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:              cfg.ListenAddr,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		Handler:           mux,
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "ok")
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
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
