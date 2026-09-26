//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/bookings"
	"github.com/PandaX185/fitcore/internal/platform/postgres"
)

func TestBookingRepositoryLifecycle(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewBookingRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	memberID := createTestMember(t, db, branchID)
	memberID2 := createTestMember(t, db, branchID)
	classID := createTestClass(t, db, branchID, 2)

	if _, err := repo.GetByID(ctx, uuid.New()); !errors.Is(err, bookings.ErrNotFound) {
		t.Fatalf("GetByID missing = %v, want ErrNotFound", err)
	}

	now := time.Now().UTC()
	id1 := uuid.New()
	if err := repo.Create(ctx, &bookings.Booking{ID: id1, ClassID: classID, MemberID: memberID, Status: bookings.StatusBooked, BookedAt: now, CreatedAt: now}); err != nil {
		t.Fatalf("Create first: %v", err)
	}
	cleanupTable(t, db, "class_bookings", id1)

	count, err := repo.CountActiveByClass(ctx, classID)
	if err != nil {
		t.Fatalf("CountActiveByClass: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	got, err := repo.GetByID(ctx, id1)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != bookings.StatusBooked || got.ClassID != classID || got.MemberID != memberID {
		t.Fatalf("GetByID = %+v", got)
	}

	cancelledAt := time.Now().UTC()
	got.Status = bookings.StatusCancelled
	got.CancelledAt = &cancelledAt
	if err := repo.Cancel(ctx, got); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	moot, err := repo.GetByID(ctx, id1)
	if err != nil {
		t.Fatalf("GetByID after cancel: %v", err)
	}
	if moot.Status != bookings.StatusCancelled || moot.CancelledAt == nil {
		t.Fatalf("after cancel = %+v", moot)
	}

	count, err = repo.CountActiveByClass(ctx, classID)
	if err != nil {
		t.Fatalf("CountActiveByClass: %v", err)
	}
	if count != 0 {
		t.Fatalf("count after cancel = %d, want 0", count)
	}

	id2 := uuid.New()
	if err := repo.Create(ctx, &bookings.Booking{ID: id2, ClassID: classID, MemberID: memberID2, Status: bookings.StatusBooked, BookedAt: now, CreatedAt: now}); err != nil {
		t.Fatalf("Create second: %v", err)
	}
	cleanupTable(t, db, "class_bookings", id2)

	list, err := repo.ListByClass(ctx, &bookings.ClassListQuery{ClassID: classID, Limit: 100})
	if err != nil {
		t.Fatalf("ListByClass: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListByClass = %d, want 2 (cancelled still rows)", len(list))
	}
}

func TestBookingRepositoryDuplicateAndRebook(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewBookingRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	memberID := createTestMember(t, db, branchID)
	classID := createTestClass(t, db, branchID, 10)

	now := time.Now().UTC()
	id1 := uuid.New()
	if err := repo.Create(ctx, &bookings.Booking{ID: id1, ClassID: classID, MemberID: memberID, Status: bookings.StatusBooked, BookedAt: now, CreatedAt: now}); err != nil {
		t.Fatalf("Create first: %v", err)
	}
	cleanupTable(t, db, "class_bookings", id1)

	// Same member + class while booked is refused by the partial unique index.
	err := repo.Create(ctx, &bookings.Booking{ID: uuid.New(), ClassID: classID, MemberID: memberID, Status: bookings.StatusBooked, BookedAt: now, CreatedAt: now})
	if !errors.Is(err, bookings.ErrDuplicate) {
		t.Fatalf("Create duplicate = %v, want ErrDuplicate", err)
	}

	// After cancelling, the seat frees up and the member may rebook.
	cancelledAt := time.Now().UTC()
	b := &bookings.Booking{ID: id1, Status: bookings.StatusCancelled, CancelledAt: &cancelledAt}
	if err := repo.Cancel(ctx, b); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	rebook := uuid.New()
	if err := repo.Create(ctx, &bookings.Booking{ID: rebook, ClassID: classID, MemberID: memberID, Status: bookings.StatusBooked, BookedAt: now, CreatedAt: now}); err != nil {
		t.Fatalf("Create after cancel = %v, want success (000005 rebook)", err)
	}
	cleanupTable(t, db, "class_bookings", rebook)
}

func TestBookingRepositoryCapacityCount(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewBookingRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	classID := createTestClass(t, db, branchID, 2)

	now := time.Now().UTC()
	ids := make([]uuid.UUID, 0, 2)
	for i := 0; i < 2; i++ {
		memberID := createTestMember(t, db, branchID)
		id := uuid.New()
		if err := repo.Create(ctx, &bookings.Booking{ID: id, ClassID: classID, MemberID: memberID, Status: bookings.StatusBooked, BookedAt: now, CreatedAt: now}); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
		ids = append(ids, id)
	}
	for _, id := range ids {
		cleanupTable(t, db, "class_bookings", id)
	}

	count, err := repo.CountActiveByClass(ctx, classID)
	if err != nil {
		t.Fatalf("CountActiveByClass: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2 (two booked rows)", count)
	}
}

func TestBookingRepositoryListByClassPaginates(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewBookingRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	classID := createTestClass(t, db, branchID, 10)

	base := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Microsecond)
	booked := []time.Time{base, base.Add(time.Hour), base.Add(2 * time.Hour), base.Add(3 * time.Hour)}
	ids := make([]uuid.UUID, 0, len(booked))
	for _, at := range booked {
		id := uuid.New()
		ids = append(ids, id)
		memberID := createTestMember(t, db, branchID)
		if err := repo.Create(ctx, &bookings.Booking{
			ID: id, ClassID: classID, MemberID: memberID,
			Status: bookings.StatusBooked, BookedAt: at, CreatedAt: at,
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}
		cleanupTable(t, db, "class_bookings", id)
	}

	var collected []uuid.UUID
	var afterKey string
	var afterID uuid.UUID
	for {
		res, err := repo.ListByClass(ctx, &bookings.ClassListQuery{
			ClassID: classID, Limit: 2, AfterBookedAt: afterKey, AfterID: afterID,
		})
		if err != nil {
			t.Fatalf("ListByClass: %v", err)
		}
		if len(res) == 0 {
			break
		}
		for _, b := range res {
			collected = append(collected, b.ID)
		}
		if len(res) < 2 {
			break
		}
		afterKey = res[len(res)-1].BookedAt.UTC().Format(time.RFC3339Nano)
		afterID = res[len(res)-1].ID
	}

	// The four inserted rows must appear in ascending created order.
	pos := map[uuid.UUID]int{}
	for i, id := range collected {
		pos[id] = i
	}
	prev := -1
	for _, id := range ids {
		p, ok := pos[id]
		if !ok {
			t.Fatalf("missing booking %v after paging: %v", id, collected)
		}
		if p <= prev {
			t.Fatalf("bookings out of order after paging: %v", collected)
		}
		prev = p
	}
}
