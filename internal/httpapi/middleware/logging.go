package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func RequestLog(log *slog.Logger) gin.HandlerFunc {
	if log == nil {
		log = slog.Default()
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" && c.Request != nil && c.Request.URL != nil {
			path = c.Request.URL.Path
		}
		fields := []any{
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		}
		if rid, ok := c.Get(RequestIDKey); ok {
			fields = append(fields, "request_id", rid)
		}
		// Probes and metrics scrapes are noisy; keep them at Debug.
		switch path {
		case "/healthz", "/readyz", "/metrics":
			log.Debug("http request", fields...)
			return
		}
		log.Info("http request", fields...)
	}
}
