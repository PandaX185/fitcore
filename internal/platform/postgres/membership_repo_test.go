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

	got, err := repo.ListByMember(ctx, &memberships.MemberListQuery{MemberID: memberID, Limit: 100})
	if err != nil {
		t.Fatalf("ListByMember: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListByMember returned %d, want 2", len(got))
	}
}

func TestMembershipRepositoryListByMemberPaginates(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewMembershipRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	memberID := createTestMember(t, db, branchID)
	packageID := createTestPackage(t, db)

	base := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Microsecond)
	starts := []time.Time{base, base.Add(time.Hour), base.Add(2 * time.Hour), base.Add(3 * time.Hour)}
	ids := make([]uuid.UUID, 0, len(starts))
	for _, st := range starts {
		id := uuid.New()
		ids = append(ids, id)
		if err := repo.Create(ctx, &memberships.Membership{
			ID: id, MemberID: memberID, PackageID: packageID, BranchID: branchID,
			Status: memberships.StatusFrozen, StartsAt: st,
			ExpiresAt: st.AddDate(0, 0, 30), CreatedAt: st, UpdatedAt: st,
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}
		cleanupTable(t, db, "memberships", id)
	}

	var collected []uuid.UUID
	var afterKey string
	var afterID uuid.UUID
	for {
		res, err := repo.ListByMember(ctx, &memberships.MemberListQuery{
			MemberID: memberID, Limit: 2, AfterStartsAt: afterKey, AfterID: afterID,
		})
		if err != nil {
			t.Fatalf("ListByMember: %v", err)
		}
		if len(res) == 0 {
			break
		}
		for _, m := range res {
			collected = append(collected, m.ID)
		}
		if len(res) < 2 {
			break
		}
		afterKey = res[len(res)-1].StartsAt.UTC().Format(time.RFC3339Nano)
		afterID = res[len(res)-1].ID
	}

	// The four inserted rows must appear in descending starts_on order.
	pos := map[uuid.UUID]int{}
	for i, id := range collected {
		pos[id] = i
	}
	prev := -1
	for i := len(ids) - 1; i >= 0; i-- {
		p, ok := pos[ids[i]]
		if !ok {
			t.Fatalf("missing membership %v after paging: %v", ids[i], collected)
		}
		if p <= prev {
			t.Fatalf("memberships out of order after paging: %v", collected)
		}
		prev = p
	}
}

func TestMembershipRepositoryListByMemberPaginatesTies(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewMembershipRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	memberID := createTestMember(t, db, branchID)
	packageID := createTestPackage(t, db)

	// Two rows sharing the same starts_on must still split across the page
	// boundary: the (starts_on = X AND id > Y) arm of the keyset.
	at := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Microsecond)
	first := uuid.New()
	second := uuid.New()
	if first.String() > second.String() {
		first, second = second, first
	}
	for _, id := range []uuid.UUID{first, second} {
		if err := repo.Create(ctx, &memberships.Membership{
			ID: id, MemberID: memberID, PackageID: packageID, BranchID: branchID,
			Status: memberships.StatusFrozen, StartsAt: at,
			ExpiresAt: at.AddDate(0, 0, 30), CreatedAt: at, UpdatedAt: at,
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}
		cleanupTable(t, db, "memberships", id)
	}

	page1, err := repo.ListByMember(ctx, &memberships.MemberListQuery{MemberID: memberID, Limit: 1})
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if len(page1) != 1 || page1[0].ID != first {
		t.Fatalf("page 1 = %+v, want [%v]", page1, first)
	}
	page2, err := repo.ListByMember(ctx, &memberships.MemberListQuery{
		MemberID: memberID, Limit: 1,
		AfterStartsAt: page1[0].StartsAt.UTC().Format(time.RFC3339Nano), AfterID: page1[0].ID,
	})
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != second {
		t.Fatalf("page 2 = %+v, want [%v]", page2, second)
	}
}

func TestMembershipRepositoryUpdateStatus(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewMembershipRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	memberID := createTestMember(t, db, branchID)
	packageID := createTestPackage(t, db)
	id := createTestMembership(t, db, memberID, packageID, branchID, memberships.StatusActive)

	frozen := memberships.StatusFrozen
	if err := repo.UpdateStatus(ctx, id, memberships.StatusActive, &memberships.Patch{Status: &frozen}); err != nil {
		t.Fatalf("UpdateStatus active->frozen: %v", err)
	}

	// The observed status moved on: conditioning on the stale status now
	// touches zero rows.
	if err := repo.UpdateStatus(ctx, id, memberships.StatusActive, &memberships.Patch{Status: &frozen}); !errors.Is(err, memberships.ErrNotFound) {
		t.Fatalf("UpdateStatus stale expected = %v, want ErrNotFound", err)
	}

	active := memberships.StatusActive
	if err := repo.UpdateStatus(ctx, id, memberships.StatusFrozen, &memberships.Patch{Status: &active}); err != nil {
		t.Fatalf("UpdateStatus frozen->active: %v", err)
	}
	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != memberships.StatusActive {
		t.Fatalf("status = %q, want active", got.Status)
	}

	if err := repo.UpdateStatus(ctx, uuid.New(), memberships.StatusActive, &memberships.Patch{Status: &frozen}); !errors.Is(err, memberships.ErrNotFound) {
		t.Fatalf("UpdateStatus missing = %v, want ErrNotFound", err)
	}
}

func TestMembershipRepositoryExpiryFilters(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewMembershipRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	memberID := createTestMember(t, db, branchID)
	packageID := createTestPackage(t, db)

	// A row whose time has passed but whose stored status is still active
	// must not count as live: there is no background expirer.
	now := time.Now().UTC()
	id := uuid.New()
	if err := repo.Create(ctx, &memberships.Membership{
		ID: id, MemberID: memberID, PackageID: packageID, BranchID: branchID,
		Status:   memberships.StatusActive,
		StartsAt: now.AddDate(0, 0, -60), ExpiresAt: now.AddDate(0, 0, -30),
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("Create expired-but-active: %v", err)
	}
	cleanupTable(t, db, "memberships", id)

	has, err := repo.HasActiveByMember(ctx, memberID)
	if err != nil {
		t.Fatalf("HasActiveByMember: %v", err)
	}
	if has {
		t.Fatal("HasActiveByMember = true for a time-expired membership")
	}
	if _, err := repo.FindActiveByMemberAndBranch(ctx, memberID, branchID); !errors.Is(err, memberships.ErrNotFound) {
		t.Fatalf("FindActiveByMemberAndBranch = %v, want ErrNotFound", err)
	}
}
