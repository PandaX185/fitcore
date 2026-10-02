package middleware

import (
	"context"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// redisCounter is the slice of the Redis API the shared limiter needs. It is
// satisfied by platform/redis.Client; test doubles implement it without a
// real Redis.
type redisCounter interface {
	// Incr increments the counter at key and returns the new value.
	Incr(ctx context.Context, key string) (int64, error)
	// Expire sets a TTL on key, reporting whether a key existed.
	Expire(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// RedisAuthLimiter is the multi-instance fixed-window limiter for the auth
// endpoints. The budget is stored in Redis (shared even when the API scales
// to several replicas) keyed by client IP and window. Unlike the in-memory
// limiter it has no lazy-sweep concern: every counter expires with its window.
//
// It fails open: if Redis is unreachable, auth requests proceed rather than
// being black-holed, but the outage is logged so it can be alerted on.
type RedisAuthLimiter struct {
	limit    int
	window   time.Duration
	redis    redisCounter
	disabled bool
	logger   *slog.Logger
	now      func() time.Time
}

// NewRedisAuthLimiter builds a shared limiter with the same per-IP budget and
// Retry-After contract as the in-memory one. logger may be nil.
func NewRedisAuthLimiter(limit int, window time.Duration, backend redisCounter, logger *slog.Logger, disabled bool) *RedisAuthLimiter {
	if window <= 0 {
		window = time.Minute
	}
	return &RedisAuthLimiter{
		limit:    limit,
		window:   window,
		redis:    backend,
		disabled: disabled,
		logger:   logger,
		now:      time.Now,
	}
}

// redisOpTimeout bounds every limiter round trip so a wedged Redis cannot
// stall the auth path longer than this (the limiter fails open on error).
const redisOpTimeout = 500 * time.Millisecond

const authRateKeyPrefix = "fitcore:ratelimit:auth:"

func (l *RedisAuthLimiter) check(ctx context.Context, ip string) (time.Duration, bool) {
	if l.disabled {
		return 0, true
	}
	now := l.now().UTC()
	windowSecs := int64(l.window / time.Second)
	windowStart := now.Unix() / windowSecs
	key := keyForIP(authRateKeyPrefix, ip, windowStart)

	opCtx, cancel := context.WithTimeout(ctx, redisOpTimeout)
	defer cancel()

	count, err := l.redis.Incr(opCtx, key)
	if err != nil {
		if l.logger != nil {
			l.logger.Warn("auth rate limit backend unavailable; failing open", "error", err)
		}
		return 0, true
	}
	// First hit of a window: pin the TTL so the counter is always cleanly
	// gone after two windows, even if a request straddles a rollover.
	if count == 1 {
		_, _ = l.redis.Expire(opCtx, key, 2*l.window)
	}
	if count > int64(l.limit) {
		windowEnd := time.Unix(windowStart*windowSecs+windowSecs, 0)
		remaining := time.Until(windowEnd)
		if remaining <= 0 {
			remaining = time.Second
		}
		return remaining, false
	}
	return 0, true
}

// Gin returns the same shared middleware used by the in-memory limiter.
func (l *RedisAuthLimiter) Gin() gin.HandlerFunc {
	return authLimiterGin(l, l.disabled)
}

func keyForIP(prefix, ip string, window int64) string {
	// IPs are already normalized by the trusted-proxy resolver; keep the raw
	// value as the key suffix so proxies cannot forge a multi-instance key.
	return prefix + ip + ":" + itoa(window)
}
