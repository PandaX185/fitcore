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
	"github.com/PandaX185/fitcore/internal/platform/postgres"
	"github.com/PandaX185/fitcore/internal/platform/telemetry"
)

type Deps struct {
	Logger  *slog.Logger
	Metrics *telemetry.Metrics
	DB      *postgres.DB
}

// New builds the Gin engine. Module routes are mounted as their HTTP adapters
// are implemented.
func New(deps Deps) *gin.Engine {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Metrics == nil {
		deps.Metrics = telemetry.New()
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLog(deps.Logger))
	r.Use(middleware.Metrics(deps.Metrics))

	r.GET("/healthz", healthz)
	r.GET("/readyz", readyz(deps.DB))
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(deps.Metrics.Registry, promhttp.HandlerOpts{})))
	openapi.RegisterHandlers(r, handlers.New(deps.Logger, deps.Metrics, deps.DB))
	mountDocs(r)

	return r
}

func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func readyz(db *postgres.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
