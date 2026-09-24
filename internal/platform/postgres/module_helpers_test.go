//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/classes"
	"github.com/PandaX185/fitcore/internal/modules/members"
	"github.com/PandaX185/fitcore/internal/modules/memberships"
	"github.com/PandaX185/fitcore/internal/modules/packages"
	"github.com/PandaX185/fitcore/internal/modules/trainers"
	"github.com/PandaX185/fitcore/internal/platform/postgres"
	"github.com/PandaX185/fitcore/internal/testutil"
)

// testutilDB is a terse alias for the integration DB helper.
func testutilDB(t *testing.T) *postgres.DB {
	t.Helper()
	return testutil.DB(t)
}

// assertTimeEqual compares instants within microsecond precision, the maximum
// PostgreSQL timestamps can round-trip (nanoseconds are truncated).
func assertTimeEqual(t *testing.T, got, want time.Time) {
	t.Helper()
	if got.Sub(want).Abs() >= time.Microsecond {
		t.Fatalf("time = %v (loc %s), want %v", got, got.Location(), want)
	}
}

// cleanupTable deletes a row from any table by id, working around the
// unexported GORM row models in the postgres package.
func cleanupTable(t *testing.T, db *postgres.DB, table string, id uuid.UUID) {
	t.Helper()
	t.Cleanup(func() {
		_ = db.Gorm().Exec("DELETE FROM "+table+" WHERE id = ?", id).Error
	})
}

func createTestMember(t *testing.T, db *postgres.DB, branchID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	repo := postgres.NewMemberRepository(db)
	if err := repo.Create(ctx, &members.Member{
		ID: id, BranchID: branchID, Name: "Smoke Member", Email: uuid.NewString()[:8] + "@example.com",
	}); err != nil {
		t.Fatalf("create member: %v", err)
	}
	cleanupTable(t, db, "members", id)
	return id
}

func createTestPackage(t *testing.T, db *postgres.DB) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	repo := postgres.NewPackageRepository(db)
	if err := repo.Create(ctx, &packages.Package{
		ID: id, Name: "Test Package " + uuid.NewString()[:8], DurationDays: 30,
		PriceCents: 25000, Currency: "BHD", Active: true,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create package: %v", err)
	}
	cleanupTable(t, db, "membership_packages", id)
	return id
}

func createTestMembership(t *testing.T, db *postgres.DB, memberID, packageID, branchID uuid.UUID, status memberships.Status) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	now := time.Now().UTC()
	repo := postgres.NewMembershipRepository(db)
	if err := repo.Create(ctx, &memberships.Membership{
		ID: id, MemberID: memberID, PackageID: packageID, BranchID: branchID,
		Status: status, StartsAt: now, ExpiresAt: now.AddDate(0, 0, 30),
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create membership: %v", err)
	}
	cleanupTable(t, db, "memberships", id)
	return id
}

func createTestTrainer(t *testing.T, db *postgres.DB, branchID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	repo := postgres.NewTrainerRepository(db)
	if err := repo.Create(ctx, &trainers.Trainer{
		ID: id, BranchID: branchID, Name: "Test Trainer", Email: uuid.NewString()[:8] + "@example.com",
		Active: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create trainer: %v", err)
	}
	cleanupTable(t, db, "trainers", id)
	return id
}

func createTestClass(t *testing.T, db *postgres.DB, branchID uuid.UUID, capacity int) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	now := time.Now().UTC().Add(24 * time.Hour)
	repo := postgres.NewClassRepository(db)
	if err := repo.Create(ctx, &classes.Class{
		ID: id, BranchID: branchID, Name: "Test Class", Capacity: capacity,
		StartsAt: now, EndsAt: now.Add(time.Hour), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create class: %v", err)
	}
	cleanupTable(t, db, "classes", id)
	return id
}
