package middleware

import (
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// authRateLimit is the shared fixed-window budget for the credential-bearing
// auth endpoints: 60 requests per minute per client IP across /auth/login and
// /auth/refresh. A shared window (rather than per-endpoint) keeps password
// guessing and token-replay probing under one ceiling.
const (
	authRateLimit  = 60
	authRateWindow = time.Minute
)

// RateLimiter is a per-IP fixed-window rate limiter. Buckets expire lazily on
// access (no background goroutine): an expired bucket is reset on next use,
// and expired entries are swept opportunistically so the map cannot grow
// without bound.
type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	buckets map[string]*rateBucket
	now     func() time.Time
}

type rateBucket struct {
	count int
	reset time.Time
}

// NewRateLimiter builds a limiter allowing limit requests per window per IP.
// A non-positive limit denies everything; a non-positive window falls back
// to one minute.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimiter{
		limit:   limit,
		window:  window,
		buckets: map[string]*rateBucket{},
		now:     time.Now,
	}
}

// NewAuthRateLimiter builds the limiter guarding the /auth/* endpoints.
func NewAuthRateLimiter() *RateLimiter {
	return NewRateLimiter(authRateLimit, authRateWindow)
}

// Gin returns middleware that rate-limits only the credential-bearing auth
// endpoints (login + refresh share one budget per IP). Other routes pass
// through untouched.
func (l *RateLimiter) Gin() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path != "/auth/login" && path != "/auth/refresh" {
			c.Next()
			return
		}
		ip := c.ClientIP()
		retryAfter, ok := l.allow(ip)
		if !ok {
			c.Header("Retry-After", itoaSeconds(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": gin.H{"message": "rate limit exceeded", "code": "rate_limited"}})
			return
		}
		c.Next()
	}
}

// allow records one request from ip. It reports the remaining cooldown and
// whether the request is within budget.
func (l *RateLimiter) allow(ip string) (time.Duration, bool) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	// Opportunistic lazy expiry: drop buckets whose window already passed so
	// idle IPs do not pin memory.
	for key, b := range l.buckets {
		if !now.Before(b.reset) {
			delete(l.buckets, key)
		}
	}

	b, ok := l.buckets[ip]
	if !ok || !now.Before(b.reset) {
		l.buckets[ip] = &rateBucket{count: 1, reset: now.Add(l.window)}
		return 0, true
	}
	if b.count >= l.limit {
		return b.reset.Sub(now), false
	}
	b.count++
	return 0, true
}

// itoaSeconds formats d as whole seconds, rounding up with a floor of 1 so a
// Retry-After of "0" is never emitted.
func itoaSeconds(d time.Duration) string {
	s := int64(math.Ceil(d.Seconds()))
	if s < 1 {
		s = 1
	}
	if s > 1<<62 {
		s = 1 << 62
	}
	return itoa(s)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
