package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/PandaX185/fitcore/internal/httpapi/middleware"
)

func rateEngine(l *middleware.RateLimiter) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(l.Gin())
	r.POST("/auth/login", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/auth/refresh", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/branches", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func postIP(r *gin.Engine, target, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, target, nil)
	// ClientIP in tests resolves from RemoteAddr when no proxy headers apply.
	req.RemoteAddr = ip + ":1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestRateLimiterExhaustsBudget pins the fixed-window contract: after the
// budget is spent across /auth/login + /auth/refresh, further requests get
// 429 with a Retry-After header, while other routes stay untouched.
func TestRateLimiterExhaustsBudget(t *testing.T) {
	r := rateEngine(middleware.NewRateLimiter(3, time.Minute))

	for i := 0; i < 3; i++ {
		if got := postIP(r, "/auth/login", "10.0.0.1").Code; got != http.StatusOK {
			t.Fatalf("request %d = %d, want 200", i+1, got)
		}
	}
	// The shared budget counts refresh too: the next request, on either
	// endpoint, is over budget.
	w := postIP(r, "/auth/refresh", "10.0.0.1")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("over-budget status = %d, want 429", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("429 response carries no Retry-After header")
	}

	// A different IP has its own budget; non-auth routes are not limited.
	if got := postIP(r, "/auth/login", "10.0.0.2").Code; got != http.StatusOK {
		t.Fatalf("other IP status = %d, want 200", got)
	}
	req := httptest.NewRequest(http.MethodGet, "/branches", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code != http.StatusOK {
		t.Fatalf("non-auth route status = %d, want 200", w2.Code)
	}
}

// TestRateLimiterWindowResets pins lazy window expiry: once the window
// passes, the same IP is admitted again without any background cleanup.
func TestRateLimiterWindowResets(t *testing.T) {
	r := rateEngine(middleware.NewRateLimiter(1, 50*time.Millisecond))

	if got := postIP(r, "/auth/login", "10.0.0.9").Code; got != http.StatusOK {
		t.Fatalf("first status = %d, want 200", got)
	}
	if got := postIP(r, "/auth/login", "10.0.0.9").Code; got != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", got)
	}
	time.Sleep(60 * time.Millisecond)
	if got := postIP(r, "/auth/login", "10.0.0.9").Code; got != http.StatusOK {
		t.Fatalf("post-window status = %d, want 200", got)
	}
}
