package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/PandaX185/fitcore/internal/platform/telemetry"
)

// Metrics labels requests by the matched route so cardinality stays bounded;
// unmatched requests fall back to "unmatched".
func Metrics(m *telemetry.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		m.ObserveHTTPRequest(c.Request.Method, path, c.Writer.Status(), time.Since(start))
	}
}
