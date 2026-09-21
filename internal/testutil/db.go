//go:build integration

package testutil

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/PandaX185/fitcore/internal/platform/postgres"
)

func DSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping integration test")
	}
	return dsn
}

// DB opens the test database. The schema must already be migrated by running
// `just migrate-up` against TEST_DATABASE_URL; this helper never migrates.
func DB(t *testing.T) *postgres.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	db, err := postgres.Open(ctx, DSN(t))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
