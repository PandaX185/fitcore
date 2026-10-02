package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/PandaX185/fitcore/internal/httpapi/handlers"
	"github.com/PandaX185/fitcore/internal/httpapi/middleware"
	"github.com/PandaX185/fitcore/internal/httpapi/openapi"
	"github.com/PandaX185/fitcore/internal/modules/auth"
	"github.com/PandaX185/fitcore/internal/platform/httpx"
	"github.com/PandaX185/fitcore/internal/platform/postgres"
	"github.com/PandaX185/fitcore/internal/platform/telemetry"
)

type Deps struct {
	Logger  *slog.Logger
	Metrics *telemetry.Metrics
	DB      *postgres.DB
	// Auth is the composed auth service (login/refresh/logout + token verify).
	Auth *auth.Service
	// Revocations rejects compromised/rotated access-token ids.
	Revocations auth.RevocationStore
}

// New builds the Gin engine. Module routes are mounted via the generated
// OpenAPI handlers plus the docs endpoints below.
func New(deps Deps) *gin.Engine {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Metrics == nil {
		deps.Metrics = telemetry.New()
	}

	r := gin.New()
	// ClientIP feeds the auth rate limiter; trust only loopback proxies so a
	// remote X-Forwarded-For cannot spoof the key.
	if err := r.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
		deps.Logger.Warn("failed to set trusted proxies", "error", err)
	}
	// BodyLimit first so oversized payloads are rejected before any other work.
	r.Use(middleware.BodyLimit(middleware.DefaultMaxBodyBytes))
	r.Use(gin.Recovery())
	// RequestID first so loggers and auth rejections share one id; the
	// rate limiter runs before AuthGuard so credential-guessing is throttled
	// even without a token.
	r.Use(middleware.RequestID())
	r.Use(middleware.RequestLog(deps.Logger))
	r.Use(middleware.Metrics(deps.Metrics))
	r.Use(middleware.NewAuthRateLimiter().Gin())
	r.Use(middleware.NewAuthGuard(deps.Auth, deps.Revocations, middleware.DefaultRegistry(), deps.Logger).Gin())

	r.GET("/healthz", healthz)
	r.GET("/readyz", readyz(deps.DB, deps.Revocations))
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(deps.Metrics.Registry, promhttp.HandlerOpts{})))
	openapi.RegisterHandlersWithOptions(r, handlers.New(deps.Logger, deps.Metrics, deps.DB, deps.Auth), openapi.GinServerOptions{ErrorHandler: httpx.CodegenErrorHandler})
	mountDocs(r)

	return r
}

func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// redisPinger is the subset of the platform redis client the readiness probe
// needs. Revocations carries it without growing Deps; stores without a Ping
// (test fakes) simply skip the redis component.
type redisPinger interface {
	Ping(ctx context.Context) error
}

func readyz(db *postgres.DB, revocations auth.RevocationStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		database := "ok"
		redis := "unconfigured"
		if pinger, ok := revocations.(redisPinger); ok {
			redis = "ok"
			pingCtx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
			if err := pinger.Ping(pingCtx); err != nil {
				redis = "unavailable"
			}
			cancel()
		}

		if db == nil {
			database = "unavailable"
		} else {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			if err := db.Ping(ctx); err != nil {
				database = "unavailable"
			}
			cancel()
		}

		if database != "ok" || redis == "unavailable" {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":   "unavailable",
				"database": database,
				"redis":    redis,
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"database": database,
			"redis":    redis,
		})
	}
}
