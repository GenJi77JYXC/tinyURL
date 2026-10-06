// Command server runs the tinyURL API service.
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

	"github.com/GenJi77JYXC/tinyurl/internal/api"
	"github.com/GenJi77JYXC/tinyurl/internal/cache"
	"github.com/GenJi77JYXC/tinyurl/internal/config"
	"github.com/GenJi77JYXC/tinyurl/internal/metrics"
	"github.com/GenJi77JYXC/tinyurl/internal/repo"
	"github.com/GenJi77JYXC/tinyurl/internal/service"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	// Root context canceled by SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// PostgreSQL is a hard dependency.
	store, err := repo.New(cfg.DatabaseURL)
	if err != nil {
		logger.Error("postgres init failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := store.Ping(pingCtx); err != nil {
		logger.Error("postgres unreachable", "error", err)
		cancel()
		os.Exit(1)
	}
	cancel()

	if err := store.Migrate(ctx); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}
	logger.Info("postgreSQL connected, migrations applied")

	// Redis is a soft dependency: log a warning and serve from PG if absent.
	linkCache := cache.New(cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB, cfg.CacheTTL, cfg.NegTTL)
	if err := linkCache.Ping(ctx); err != nil {
		logger.Warn("redis unreachable, starting in degraded mode (cache bypassed)", "error", err)
	} else {
		logger.Info("redis connected", "addr", cfg.RedisAddr)
	}
	defer linkCache.Close()

	metrics.Register()
	go metrics.CollectRuntime(ctx, linkCache, 15*time.Second)

	svc := service.New(store, linkCache, cfg.BaseURL)
	handler := api.NewHandler(svc)

	limiter := api.NewIPLimiter(cfg.RateRPS, cfg.RateBurst)
	sweeperStop := make(chan struct{})
	go limiter.RunSweeper(time.Minute, 10*time.Minute, sweeperStop)

	router := api.NewRouter(handler, cfg, limiter)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", cfg.HTTPAddr, "base_url", cfg.BaseURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		logger.Error("http server failed", "error", err)
		os.Exit(1)
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining connections")
	}
	close(sweeperStop)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTO)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown timed out, forcing exit", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped cleanly")
}
