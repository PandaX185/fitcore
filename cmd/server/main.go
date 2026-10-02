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
	"github.com/PandaX185/fitcore/internal/httpapi/middleware"
	"github.com/PandaX185/fitcore/internal/modules/auth"
	"github.com/PandaX185/fitcore/internal/platform/logging"
	"github.com/PandaX185/fitcore/internal/platform/postgres"
	"github.com/PandaX185/fitcore/internal/platform/redis"
	"github.com/PandaX185/fitcore/internal/platform/telemetry"
	"github.com/gin-gonic/gin"
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

	if cfg.Env != "development" {
		gin.SetMode(gin.ReleaseMode)
	}

	db, err := postgres.OpenWithPool(ctx, cfg.DatabaseURL, cfg.MaxOpenConns, cfg.MaxIdleConns)
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

	// Auth rate limiter: a shared Redis fixed-window counter when
	// RATE_LIMIT_REDIS=true (multi-replica budget), otherwise the in-memory
	// per-instance limiter. The disabled flag (load stack only) wins either way.
	var authLimiter middleware.AuthLimiter
	if cfg.RateLimitRedis && !cfg.RateLimitDisabled {
		authLimiter = middleware.NewRedisAuthLimiter(
			middleware.DefaultAuthRateLimit, middleware.DefaultAuthRateWindow, redisClient, log, false)
	} else {
		authLimiter = middleware.NewAuthRateLimiter(cfg.RateLimitDisabled)
	}

	router := httpapi.New(httpapi.Deps{
		Logger:            log,
		Metrics:           metrics,
		DB:                db,
		Auth:              authSvc,
		Revocations:       revocations,
		RateLimitDisabled: cfg.RateLimitDisabled,
		AuthLimiter:       authLimiter,
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	srvErr := make(chan error, 1)
	go func() {
		log.Info("fitcore server listening", "addr", cfg.HTTPAddr)
		srvErr <- srv.ListenAndServe()
	}()

	// Pool sampler: 15s cadence keeps dashboards fresh without churn.
	poolTicker := time.NewTicker(15 * time.Second)
	defer poolTicker.Stop()
	go func() {
		for range poolTicker.C {
			metrics.ObservePoolStats(db.Stats())
		}
	}()

	stop := make(chan os.Signal, 2)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)

	select {
	case err := <-srvErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("http server: %w", err)
	case sig := <-stop:
		log.Info("shutting down", "signal", sig.String())
		go func() {
			<-stop
			log.Error("second signal received, forcing exit")
			os.Exit(1)
		}()
		poolTicker.Stop()
		// Drain longer than typical requests: in-flight check-ins, booking
		// transactions and login Argon2 work can exceed 10s under load.
		shutdownCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http server shutdown: %w", err)
		}
		log.Info("drained")
		return nil
	}
}
