package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/PandaX185/fitcore/internal/httpapi/middleware"
)

// TestRequestID pins generate/propagate: an incoming X-Request-ID is echoed
// untouched, while a missing one is generated (UUID) and visible both in the
// response header and the gin context.
func TestRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var seen any
	r := gin.New()
	r.Use(middleware.RequestID())
	r.GET("/ping", func(c *gin.Context) {
		seen, _ = c.Get(middleware.RequestIDKey)
		c.Status(http.StatusOK)
	})

	t.Run("propagates incoming id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set("X-Request-ID", "caller-123")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if got := w.Header().Get("X-Request-ID"); got != "caller-123" {
			t.Fatalf("response X-Request-ID = %q, want %q", got, "caller-123")
		}
		if s, ok := seen.(string); !ok || s != "caller-123" {
			t.Fatalf("context request_id = %v, want %q", seen, "caller-123")
		}
	})

	t.Run("generates missing id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		got := w.Header().Get("X-Request-ID")
		if got == "" {
			t.Fatal("response carries no X-Request-ID")
		}
		if s, ok := seen.(string); !ok || s != got {
			t.Fatalf("context request_id = %v, want header value %q", seen, got)
		}
	})
}
