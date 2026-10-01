package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDKey is the gin context key and the HTTP header name carrying the
// request id. The middleware propagates an incoming X-Request-ID and generates
// a UUIDv4 when absent, setting both the context value and the response
// header so callers can correlate logs.
const RequestIDKey = "request_id"

// RequestID generates or propagates the request id for every request. It
// must run first in the chain so loggers and auth rejections can reference
// the same id.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(RequestIDKey, id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}
