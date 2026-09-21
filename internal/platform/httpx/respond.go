package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PandaX185/fitcore/internal/platform/telemetry"
)

func JSON(c *gin.Context, status int, body any) {
	c.JSON(status, body)
}

// Error sends a JSON error response, records a metric, and logs the cause. The
// message is client-safe; the cause is logged but never returned verbatim.
func Error(c *gin.Context, log *slog.Logger, metrics *telemetry.Metrics, module, operation string, status int, message string, cause error) {
	if metrics != nil {
		metrics.RecordApplicationError(module, operation, status)
	}
	if log != nil {
		log.Warn("http request failed",
			"module", module,
			"operation", operation,
			"status", status,
			"message", message,
			"error", errString(cause),
		)
	}
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"message": message}})
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func StatusFor(err error, notFound, invalid, conflict error) int {
	switch {
	case err != nil && errors.Is(err, notFound):
		return http.StatusNotFound
	case err != nil && errors.Is(err, invalid):
		return http.StatusUnprocessableEntity
	case err != nil && errors.Is(err, conflict):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
