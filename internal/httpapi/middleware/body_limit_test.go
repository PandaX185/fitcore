package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func bodyLimitRouter(limit int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BodyLimit(limit))
	r.POST("/echo", func(c *gin.Context) {
		body, err := c.GetRawData()
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		c.Data(http.StatusOK, "application/octet-stream", body)
	})
	return r
}

func TestBodyLimitAllowsSmallBodies(t *testing.T) {
	r := bodyLimitRouter(0) // default 1 MiB
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader([]byte(`{"a":1}`)))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if w.Body.String() != `{"a":1}` {
		t.Fatalf("body = %q, want passthrough", w.Body.String())
	}
}

func TestBodyLimitRejectsOversizedBodies(t *testing.T) {
	r := bodyLimitRouter(16)
	big := bytes.Repeat([]byte("x"), 17)

	// Declared ContentLength over the limit.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(big))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
	}

	// Unknown length (chunked-style) over the limit.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(big))
	req.ContentLength = -1
	r.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("chunked status = %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
	}
}
