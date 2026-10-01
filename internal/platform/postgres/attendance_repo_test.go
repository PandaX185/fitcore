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
		out := now.Add(time.Minute)
		if err := repo.Close(ctx, &attendance.Attendance{ID: id, CheckedOutAt: &out}); err != nil {
			t.Fatalf("Close %d: %v", i, err)
		}
		ids = append(ids, id)
	}
	for _, id := range ids {
		cleanupTable(t, db, "attendance", id)
	}

	got, err := repo.ListByMember(ctx, &attendance.MemberListQuery{MemberID: memberID, Limit: 100})
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

func TestAttendanceRepositoryListByMemberPaginates(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewAttendanceRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	memberID := createTestMember(t, db, branchID)
	packageID := createTestPackage(t, db)
	membershipID := createTestMembership(t, db, memberID, packageID, branchID, memberships.StatusActive)

	base := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Microsecond)
	checkins := []time.Time{base, base.Add(time.Hour), base.Add(2 * time.Hour), base.Add(3 * time.Hour)}
	ids := make([]uuid.UUID, 0, len(checkins))
	for _, at := range checkins {
		id := uuid.New()
		ids = append(ids, id)
		if err := repo.Create(ctx, &attendance.Attendance{
			ID: id, MemberID: memberID, BranchID: branchID, MembershipID: membershipID,
			CheckedInAt: at, CreatedAt: at,
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}
		// Only one open visit may exist per member (uq_attendance_open_member);
		// close each row so the next insert is legal. Closed rows still page.
		out := at.Add(time.Minute)
		if err := repo.Close(ctx, &attendance.Attendance{ID: id, CheckedOutAt: &out}); err != nil {
			t.Fatalf("Close: %v", err)
		}
		cleanupTable(t, db, "attendance", id)
	}

	var collected []uuid.UUID
	var afterKey string
	var afterID uuid.UUID
	for {
		res, err := repo.ListByMember(ctx, &attendance.MemberListQuery{
			MemberID: memberID, Limit: 2, AfterCheckedInAt: afterKey, AfterID: afterID,
		})
		if err != nil {
			t.Fatalf("ListByMember: %v", err)
		}
		if len(res) == 0 {
			break
		}
		for _, a := range res {
			collected = append(collected, a.ID)
		}
		if len(res) < 2 {
			break
		}
		afterKey = res[len(res)-1].CheckedInAt.UTC().Format(time.RFC3339Nano)
		afterID = res[len(res)-1].ID
	}

	// The four inserted rows must appear in descending check-in order.
	pos := map[uuid.UUID]int{}
	for i, id := range collected {
		pos[id] = i
	}
	prev := -1
	for i := len(ids) - 1; i >= 0; i-- {
		p, ok := pos[ids[i]]
		if !ok {
			t.Fatalf("missing attendance %v after paging: %v", ids[i], collected)
		}
		if p <= prev {
			t.Fatalf("attendance out of order after paging: %v", collected)
		}
		prev = p
	}
}
