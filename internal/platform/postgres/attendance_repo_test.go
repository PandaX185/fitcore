//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/attendance"
	"github.com/PandaX185/fitcore/internal/modules/memberships"
	"github.com/PandaX185/fitcore/internal/platform/postgres"
)

func TestAttendanceRepositoryCreateAndGet(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewAttendanceRepository(db)
	ctx := context.Background()

	if _, err := repo.GetByID(ctx, uuid.New()); !errors.Is(err, attendance.ErrNotFound) {
		t.Fatalf("GetByID missing = %v, want ErrNotFound", err)
	}

	branchID := createTestBranch(t, db)
	memberID := createTestMember(t, db, branchID)
	packageID := createTestPackage(t, db)
	membershipID := createTestMembership(t, db, memberID, packageID, branchID, memberships.StatusActive)

	id := uuid.New()
	now := time.Now().UTC()
	a := &attendance.Attendance{
		ID: id, MemberID: memberID, BranchID: branchID, MembershipID: membershipID,
		CheckedInAt: now, CheckedOutAt: nil, CreatedAt: now,
	}
	if err := repo.Create(ctx, a); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupTable(t, db, "attendance", id)

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != id || got.MemberID != memberID || got.BranchID != branchID {
		t.Fatalf("GetByID = %+v", got)
	}
	if got.CheckedOutAt != nil {
		t.Fatalf("unexpected checkout: %+v", got)
	}
}

func TestAttendanceRepositoryCheckInOut(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewAttendanceRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	memberID := createTestMember(t, db, branchID)
	packageID := createTestPackage(t, db)
	membershipID := createTestMembership(t, db, memberID, packageID, branchID, memberships.StatusActive)

	id := uuid.New()
	now := time.Now().UTC()
	if err := repo.Create(ctx, &attendance.Attendance{ID: id, MemberID: memberID, BranchID: branchID, MembershipID: membershipID, CheckedInAt: now, CreatedAt: now}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupTable(t, db, "attendance", id)

	open, err := repo.FindOpenByMember(ctx, memberID)
	if err != nil {
		t.Fatalf("FindOpenByMember: %v", err)
	}
	if open.ID != id {
		t.Fatalf("open = %v, want %v", open.ID, id)
	}

	closeAt := now.Add(time.Hour)
	open.CheckedOutAt = &closeAt
	if err := repo.Close(ctx, open); err != nil {
		t.Fatalf("Close: %v", err)
	}
	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID after close: %v", err)
	}
	if got.CheckedOutAt == nil {
		t.Fatalf("checkout missing: %+v", got)
	}

	// Once closed the record is no longer "open" — Close rows=0 -> ErrNotFound.
	if err := repo.Close(ctx, open); !errors.Is(err, attendance.ErrNotFound) {
		t.Fatalf("Close closed = %v, want ErrNotFound", err)
	}
	if _, err := repo.FindOpenByMember(ctx, memberID); !errors.Is(err, attendance.ErrNotFound) {
		t.Fatalf("FindOpen after close = %v, want ErrNotFound", err)
	}
}

func TestAttendanceRepositoryListByMember(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewAttendanceRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	memberID := createTestMember(t, db, branchID)
	packageID := createTestPackage(t, db)
	membershipID := createTestMembership(t, db, memberID, packageID, branchID, memberships.StatusActive)

	ids := make([]uuid.UUID, 0, 2)
	for i := 0; i < 2; i++ {
		id := uuid.New()
		now := time.Now().UTC().Add(time.Duration(-i) * time.Hour)
		if err := repo.Create(ctx, &attendance.Attendance{ID: id, MemberID: memberID, BranchID: branchID, MembershipID: membershipID, CheckedInAt: now, CreatedAt: now}); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
		ids = append(ids, id)
	}
	for _, id := range ids {
		cleanupTable(t, db, "attendance", id)
	}

	got, err := repo.ListByMember(ctx, memberID)
	if err != nil {
		t.Fatalf("ListByMember: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListByMember = %d, want 2", len(got))
	}
	if got[0].CheckedInAt.Before(got[1].CheckedInAt) {
		t.Fatalf("expected most-recent-first, got %v then %v", got[0].CheckedInAt, got[1].CheckedInAt)
	}
}
