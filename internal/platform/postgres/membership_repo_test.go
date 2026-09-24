//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/memberships"
	"github.com/PandaX185/fitcore/internal/platform/postgres"
)

func TestMembershipRepositoryCreateAndGet(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewMembershipRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	memberID := createTestMember(t, db, branchID)
	packageID := createTestPackage(t, db)

	if _, err := repo.GetByID(ctx, uuid.New()); !errors.Is(err, memberships.ErrNotFound) {
		t.Fatalf("GetByID missing = %v, want ErrNotFound", err)
	}

	id := uuid.New()
	now := time.Now().UTC()
	m := &memberships.Membership{
		ID: id, MemberID: memberID, PackageID: packageID, BranchID: branchID,
		Status: memberships.StatusFrozen, StartsAt: now, ExpiresAt: now.AddDate(0, 0, 30),
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Create(ctx, m); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupTable(t, db, "memberships", id)

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != id || got.MemberID != memberID || got.PackageID != packageID || got.BranchID != branchID {
		t.Fatalf("GetByID = %+v", got)
	}
	if got.Status != memberships.StatusFrozen {
		t.Fatalf("status = %q, want frozen", got.Status)
	}
	assertTimeEqual(t, got.StartsAt, now)
	assertTimeEqual(t, got.ExpiresAt, now.AddDate(0, 0, 30))
}

func TestMembershipRepositoryDuplicateActive(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewMembershipRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	memberID := createTestMember(t, db, branchID)
	packageID := createTestPackage(t, db)

	now := time.Now().UTC()
	if err := repo.Create(ctx, &memberships.Membership{
		ID: uuid.New(), MemberID: memberID, PackageID: packageID, BranchID: branchID,
		Status: memberships.StatusActive, StartsAt: now, ExpiresAt: now.AddDate(0, 0, 30),
	}); err != nil {
		t.Fatalf("Create first: %v", err)
	}

	dupErr := repo.Create(ctx, &memberships.Membership{
		ID: uuid.New(), MemberID: memberID, PackageID: packageID, BranchID: branchID,
		Status: memberships.StatusActive, StartsAt: now, ExpiresAt: now.AddDate(0, 0, 30),
	})
	if !errors.Is(dupErr, memberships.ErrDuplicateActive) {
		t.Fatalf("Create duplicate active = %v, want ErrDuplicateActive", dupErr)
	}
}

func TestMembershipRepositoryUpdate(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewMembershipRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	memberID := createTestMember(t, db, branchID)
	packageID := createTestPackage(t, db)
	id := createTestMembership(t, db, memberID, packageID, branchID, memberships.StatusActive)

	frozen := memberships.StatusFrozen
	if err := repo.Update(ctx, id, &memberships.Patch{Status: &frozen}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != memberships.StatusFrozen {
		t.Fatalf("after update = %q, want frozen", got.Status)
	}

	if err := repo.Update(ctx, uuid.New(), &memberships.Patch{Status: &frozen}); !errors.Is(err, memberships.ErrNotFound) {
		t.Fatalf("Update missing = %v, want ErrNotFound", err)
	}
}

func TestMembershipRepositoryActiveQueries(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewMembershipRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	memberID := createTestMember(t, db, branchID)
	packageID := createTestPackage(t, db)

	activeID := createTestMembership(t, db, memberID, packageID, branchID, memberships.StatusActive)
	_ = activeID

	has, err := repo.HasActiveByMember(ctx, memberID)
	if err != nil || !has {
		t.Fatalf("HasActiveByMember = %v, %v; want true", has, err)
	}

	active, err := repo.FindActiveByMemberAndBranch(ctx, memberID, branchID)
	if err != nil {
		t.Fatalf("FindActiveByMemberAndBranch: %v", err)
	}
	if active.MemberID != memberID || active.BranchID != branchID {
		t.Fatalf("active = %+v", active)
	}

	if _, err := repo.FindActiveByMemberAndBranch(ctx, memberID, uuid.New()); !errors.Is(err, memberships.ErrNotFound) {
		t.Fatalf("wrong branch = %v, want ErrNotFound", err)
	}
	if _, err := repo.FindActiveByMemberAndBranch(ctx, uuid.New(), branchID); !errors.Is(err, memberships.ErrNotFound) {
		t.Fatalf("unknown member = %v, want ErrNotFound", err)
	}
}

func TestMembershipRepositoryListByMember(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewMembershipRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	memberID := createTestMember(t, db, branchID)
	packageID := createTestPackage(t, db)

	createTestMembership(t, db, memberID, packageID, branchID, memberships.StatusFrozen)
	createTestMembership(t, db, memberID, packageID, branchID, memberships.StatusExpired)

	got, err := repo.ListByMember(ctx, memberID)
	if err != nil {
		t.Fatalf("ListByMember: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListByMember returned %d, want 2", len(got))
	}
}
