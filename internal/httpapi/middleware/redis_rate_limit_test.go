package middleware

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeRedisCounter is an in-memory stand-in for the Redis backend used by the
// shared limiter. It behaves like INCR/EXPIRE (with no expiration sweep).
type fakeRedisCounter struct {
	mu   sync.Mutex
	keys map[string]int64
	err  atomic.Value
}

func newFakeRedisCounter() *fakeRedisCounter {
	return &fakeRedisCounter{keys: map[string]int64{}}
}

func (f *fakeRedisCounter) Incr(_ context.Context, key string) (int64, error) {
	if v, ok := f.err.Load().(error); ok {
		return 0, v
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.keys[key]++
	return f.keys[key], nil
}

func (f *fakeRedisCounter) Expire(_ context.Context, key string, ttl time.Duration) (bool, error) {
	if v, ok := f.err.Load().(error); ok {
		return false, v
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	_, existed := f.keys[key]
	return existed, nil
}

func (f *fakeRedisCounter) setErr(err error) { f.err.Store(err) }

func TestRedisAuthLimiterSharedBudget(t *testing.T) {
	backend := newFakeRedisCounter()
	limiter := NewRedisAuthLimiter(2, time.Minute, backend, nil, false)

	for i := 0; i < 2; i++ {
		if _, ok := limiter.check(context.Background(), "203.0.113.9"); !ok {
			t.Fatalf("request %d: within budget should be allowed", i+1)
		}
	}
	// Third request from the same IP inside the window is denied.
	retryAfter, ok := limiter.check(context.Background(), "203.0.113.9")
	if ok {
		t.Fatal("third request: expected 429")
	}
	if retryAfter <= 0 || retryAfter > time.Minute {
		t.Fatalf("retry-after = %v, want 0 < d <= 1m", retryAfter)
	}

	// A different IP has its own budget.
	if _, ok := limiter.check(context.Background(), "203.0.113.10"); !ok {
		t.Fatal("different IP: should be allowed")
	}
}

func TestRedisAuthLimiterFailsOpen(t *testing.T) {
	backend := newFakeRedisCounter()
	backend.setErr(errors.New("connection refused"))
	limiter := NewRedisAuthLimiter(1, time.Minute, backend, nil, false)

	// Redis down => requests pass, never 429.
	for i := 0; i < 5; i++ {
		if _, ok := limiter.check(context.Background(), "203.0.113.9"); !ok {
			t.Fatalf("request %d: fail-open should allow when backend errors", i+1)
		}
	}
}

func TestRedisAuthLimiterDisabled(t *testing.T) {
	backend := newFakeRedisCounter()
	limiter := NewRedisAuthLimiter(1, time.Minute, backend, nil, true)

	for i := 0; i < 10; i++ {
		if _, ok := limiter.check(context.Background(), "203.0.113.9"); !ok {
			t.Fatal("disabled limiter should never deny")
		}
	}
	// No keys should ever have been written.
	if len(backend.keys) != 0 {
		t.Fatalf("disabled limiter wrote keys: %d", len(backend.keys))
	}
}

func TestKeyForIPIncludesWindow(t *testing.T) {
	a := keyForIP("p:", "203.0.113.9", 100)
	b := keyForIP("p:", "203.0.113.9", 101)
	if a == b {
		t.Fatalf("keys %q and %q must differ across windows", a, b)
	}
	if !strings.HasPrefix(a, "p:203.0.113.9:") {
		t.Fatalf("unexpected key shape %q", a)
	}
}
