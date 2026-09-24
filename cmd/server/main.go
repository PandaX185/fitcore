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

	"github.com/PandaX185/fitcore/internal/config"
	"github.com/PandaX185/fitcore/internal/httpapi"
	"github.com/PandaX185/fitcore/internal/modules/auth"
	"github.com/PandaX185/fitcore/internal/platform/logging"
	"github.com/PandaX185/fitcore/internal/platform/postgres"
	"github.com/PandaX185/fitcore/internal/platform/redis"
	"github.com/PandaX185/fitcore/internal/platform/telemetry"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fitcore server failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logging.New(cfg.LogLevel)
	ctx := context.Background()

	db, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	redisClient, err := redis.Open(ctx, cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("open redis: %w", err)
	}
	defer func() { _ = redisClient.Close() }()

	issuer, err := auth.NewTokenIssuer([]byte(cfg.TokenSecret), cfg.AccessTokenTTL)
	if err != nil {
		return fmt.Errorf("init token issuer: %w", err)
	}
	authRepo := postgres.NewAuthRepository(db)
	revocations := redis.NewRevocationStore(redisClient)
	authSvc := auth.NewService(authRepo, authRepo, revocations, issuer, cfg.AccessTokenTTL, cfg.RefreshTTL)

	metrics := telemetry.New()
	router := httpapi.New(httpapi.Deps{
		Logger:      log,
		Metrics:     metrics,
		DB:          db,
		Auth:        authSvc,
		Revocations: revocations,
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	_ = srv

	srvErr := make(chan error, 1)
	go func() {
		log.Info("fitcore server listening", "addr", cfg.HTTPAddr)
		srvErr <- srv.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-srvErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("http server: %w", err)
	case sig := <-stop:
		log.Info("shutting down", "signal", sig.String())
		shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
