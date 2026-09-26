//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/auth"
	"github.com/PandaX185/fitcore/internal/modules/staff"
	"github.com/PandaX185/fitcore/internal/platform/postgres"
)

func TestStaffRepositoryCreateAndGet(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewStaffRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	if _, err := repo.GetByID(ctx, uuid.New()); !errors.Is(err, staff.ErrNotFound) {
		t.Fatalf("GetByID missing = %v, want ErrNotFound", err)
	}

	id := uuid.New()
	s := &staff.Staff{
		ID: id, BranchID: branchID, Name: "Staff One", Email: uuid.NewString()[:8] + "@example.com",
		Phone: "123", Permissions: []auth.Permission{auth.PermBranchesRead, auth.PermMembersRead}, Active: true,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupTable(t, db, "staff", id)

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != id || got.BranchID != branchID || got.Name != s.Name || got.Email != s.Email || !got.Active {
		t.Fatalf("GetByID = %+v, want %+v", got, s)
	}
	if len(got.Permissions) != 2 {
		t.Fatalf("permissions = %v", got.Permissions)
	}
}

func TestStaffRepositoryDuplicateEmail(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewStaffRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	email := uuid.NewString()[:8] + "@example.com"

	id1 := uuid.New()
	if err := repo.Create(ctx, &staff.Staff{ID: id1, BranchID: branchID, Name: "One", Email: email}); err != nil {
		t.Fatalf("Create first: %v", err)
	}
	cleanupTable(t, db, "staff", id1)

	err := repo.Create(ctx, &staff.Staff{ID: uuid.New(), BranchID: branchID, Name: "Two", Email: email})
	if !errors.Is(err, staff.ErrDuplicateEmail) {
		t.Fatalf("Create dup = %v, want ErrDuplicateEmail", err)
	}
}

func TestStaffRepositoryUpdate(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewStaffRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	id := uuid.New()
	if err := repo.Create(ctx, &staff.Staff{
		ID: id, BranchID: branchID, Name: "Old", Email: uuid.NewString()[:8] + "@example.com",
		Permissions: []auth.Permission{auth.PermBranchesRead},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupTable(t, db, "staff", id)

	name := "New"
	perms := []auth.Permission{auth.PermBranchesRead, auth.PermMembersUpdate}
	active := false
	if err := repo.Update(ctx, id, &staff.Patch{Name: &name, Permissions: &perms, Active: &active}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "New" || got.Active || len(got.Permissions) != 2 {
		t.Fatalf("after update = %+v", got)
	}

	if err := repo.Update(ctx, uuid.New(), &staff.Patch{Name: &name}); !errors.Is(err, staff.ErrNotFound) {
		t.Fatalf("Update missing = %v, want ErrNotFound", err)
	}
}

func TestStaffRepositoryListByBranch(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewStaffRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	id := uuid.New()
	if err := repo.Create(ctx, &staff.Staff{ID: id, BranchID: branchID, Name: "Branch Staff", Email: uuid.NewString()[:8] + "@example.com"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupTable(t, db, "staff", id)

	got, err := repo.ListByBranch(ctx, &staff.BranchListQuery{BranchID: branchID, Limit: 100})
	if err != nil {
		t.Fatalf("ListByBranch: %v", err)
	}
	if len(got) < 1 {
		t.Fatalf("ListByBranch returned %d, want >= 1", len(got))
	}
	other, err := repo.ListByBranch(ctx, &staff.BranchListQuery{BranchID: uuid.New(), Limit: 100})
	if err != nil {
		t.Fatalf("ListByBranch(other): %v", err)
	}
	for _, s := range other {
		if s.BranchID == branchID {
			t.Fatalf("other branch leaked row: %+v", s)
		}
	}
}

func TestStaffRepositoryListByBranchPaginates(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewStaffRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)

	tag := uuid.NewString()[:8]
	names := []string{"Plumb " + tag, "Quill " + tag, "Ridge " + tag, "Slate " + tag}
	for _, n := range names {
		id := uuid.New()
		if err := repo.Create(ctx, &staff.Staff{ID: id, BranchID: branchID, Name: n, Email: id.String()[:8] + "@example.com"}); err != nil {
			t.Fatalf("Create(%s): %v", n, err)
		}
		cleanupTable(t, db, "staff", id)
	}

	var collected []string
	var afterName string
	var afterID uuid.UUID
	for {
		res, err := repo.ListByBranch(ctx, &staff.BranchListQuery{BranchID: branchID, Limit: 2, AfterName: afterName, AfterID: afterID})
		if err != nil {
			t.Fatalf("ListByBranch: %v", err)
		}
		if len(res) == 0 {
			break
		}
		for _, s := range res {
			collected = append(collected, s.Name)
		}
		afterName = res[len(res)-1].Name
		afterID = res[len(res)-1].ID
		if len(res) < 2 {
			break
		}
	}

	// Page boundaries must produce every inserted row, each row in name order
	// relative to the others (the database is shared, so other rows may
	// appear between them).
	pos := map[string]int{}
	for i, n := range collected {
		pos[n] = i
	}
	prev := -1
	for _, n := range names {
		p, ok := pos[n]
		if !ok {
			t.Fatalf("missing row %q after paging: %v", n, collected)
		}
		if p <= prev {
			t.Fatalf("rows out of order after paging: %v", collected)
		}
		prev = p
	}
}
