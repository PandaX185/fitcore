//go:build integration

package postgres_test

import (
	"testing"

	"github.com/PandaX185/fitcore/internal/testutil"
)

// TestSchemaPresent guards the precondition that migrations were applied
// manually before running integration tests.
func TestSchemaPresent(t *testing.T) {
	db := testutil.DB(t)

	for _, table := range []string{"branches", "members", "memberships", "classes", "invoices"} {
		if !db.Gorm().Migrator().HasTable(table) {
			t.Fatalf("table %q is missing; run `just migrate-up` first", table)
		}
	}
}
