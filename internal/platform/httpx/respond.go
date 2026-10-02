package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PandaX185/fitcore/internal/httpapi/middleware"
	"github.com/PandaX185/fitcore/internal/platform/telemetry"
)

func JSON(c *gin.Context, status int, body any) {
	c.JSON(status, body)
}

// Error sends a JSON error response, records a metric, and logs the cause. The
// message is client-safe; the cause is logged but never returned verbatim.
// The body matches the api/openapi.yaml Error schema:
// {"error": <message>, "code": <code>}.
func Error(c *gin.Context, log *slog.Logger, metrics *telemetry.Metrics, module, operation string, status int, message string, cause error) {
	if metrics != nil {
		metrics.RecordApplicationError(module, operation, status)
		if status >= 500 {
			metrics.RecordDatabaseError(module, operation)
		}
	}
	if log != nil {
		path := c.FullPath()
		if path == "" && c.Request != nil && c.Request.URL != nil {
			path = c.Request.URL.Path
		}
		fields := []any{
			"module", module,
			"operation", operation,
			"status", status,
			"message", message,
			"error", errString(cause),
			"method", c.Request.Method,
			"path", path,
		}
		if rid, ok := c.Get(middleware.RequestIDKey); ok {
			fields = append(fields, "request_id", rid)
		}
		log.Warn("http request failed", fields...)
	}
	c.AbortWithStatusJSON(status, gin.H{"error": message, "code": codeForStatus(status)})
}

// CodegenErrorHandler adapts the error envelope to the oapi-codegen
// GinServerOptions.ErrorHandler signature so malformed path/query parameters
// rejected by the generated router use the same {"error", "code"} body.
func CodegenErrorHandler(c *gin.Context, err error, status int) {
	message := "invalid request"
	if err != nil {
		message = err.Error()
	}
	c.AbortWithStatusJSON(status, gin.H{"error": message, "code": codeForStatus(status)})
}

// codeForStatus maps an HTTP status to the stable machine-readable code
// carried by the Error schema.
func codeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	default:
		return "internal_error"
	}
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
		return http.StatusBadRequest
	case err != nil && errors.Is(err, conflict):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
