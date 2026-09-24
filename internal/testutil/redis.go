//go:build integration

package testutil

import (
	"context"
	"os"
	"testing"
	"time"

	re "github.com/PandaX185/fitcore/internal/platform/redis"
)

// RedisURL returns the configured test Redis instance.
func RedisURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL is not set; skipping integration test")
	}
	return url
}

// Redis opens the test Redis used by integration tests. The store is emptied
// and closed automatically.
func Redis(t *testing.T) *re.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	c, err := re.Open(ctx, RedisURL(t))
	if err != nil {
		t.Fatalf("open redis: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	if err := c.FlushDB(ctx); err != nil {
		t.Fatalf("flush test redis: %v", err)
	}
	return c
}
