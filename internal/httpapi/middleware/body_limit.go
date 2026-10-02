package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// DefaultMaxBodyBytes caps JSON request bodies at 1 MiB.
const DefaultMaxBodyBytes = 1 << 20

// BodyLimit rejects request bodies larger than limit with 413. It wraps the
// body in http.MaxBytesReader so slow/chunked senders are cut off at the same
// bound, then buffers up to limit+1 bytes to detect excess deterministically
// before the handler runs. A non-positive limit selects DefaultMaxBodyBytes.
func BodyLimit(limit int64) gin.HandlerFunc {
	if limit <= 0 {
		limit = DefaultMaxBodyBytes
	}
	return func(c *gin.Context) {
		if c.Request.Body == nil {
			c.Next()
			return
		}
		if c.Request.ContentLength > limit {
			abortBodyTooLarge(c)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, limit+1))
		if err != nil || int64(len(body)) > limit {
			abortBodyTooLarge(c)
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Next()
	}
}

func abortBodyTooLarge(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large", "code": "request_too_large"})
}
