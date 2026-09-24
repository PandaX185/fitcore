//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/branches"
	"github.com/PandaX185/fitcore/internal/modules/members"
	"github.com/PandaX185/fitcore/internal/platform/postgres"
	"github.com/PandaX185/fitcore/internal/testutil"
)

func TestMemberRepositoryCreateAndGet(t *testing.T) {
	db := testutil.DB(t)
	repo := postgres.NewMemberRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	if !errors.Is(err, members.ErrNotFound) {
		t.Fatalf("GetByID missing = %v, want ErrNotFound", err)
	}

	branchID := createTestBranch(t, db)

	id := uuid.New()
	m := &members.Member{
		ID: id, BranchID: branchID, Name: "Ada Lovelace", Email: "ada@example.com",
		Phone: "12345", Status: members.StatusActive,
	}
	if err := repo.Create(ctx, m); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Gorm().WithContext(ctx).Unscoped().Delete(&members.Member{}, "id = ?", id).Error
	})

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != id || got.BranchID != branchID || got.Name != m.Name || got.Email != m.Email {
		t.Fatalf("GetByID = %+v, want %+v", got, m)
	}
	if got.Phone != m.Phone || got.Status != m.Status {
		t.Fatalf("phone/status = %q/%q, want %q/%q", got.Phone, got.Status, m.Phone, m.Status)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("timestamps not populated: %+v", got)
	}
}

func TestMemberRepositoryCreateDuplicateEmail(t *testing.T) {
	db := testutil.DB(t)
	repo := postgres.NewMemberRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)

	id1 := uuid.New()
	id2 := uuid.New()
	if err := repo.Create(ctx, &members.Member{ID: id1, BranchID: branchID, Name: "One", Email: "dup@example.com"}); err != nil {
		t.Fatalf("Create first: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Gorm().WithContext(ctx).Unscoped().Delete(&members.Member{}, "id IN ?", []uuid.UUID{id1, id2}).Error
	})

	err := repo.Create(ctx, &members.Member{ID: id2, BranchID: branchID, Name: "Two", Email: "dup@example.com"})
	if !errors.Is(err, members.ErrDuplicateEmail) {
		t.Fatalf("Create duplicate = %v, want ErrDuplicateEmail", err)
	}
}

func TestMemberRepositoryUpdate(t *testing.T) {
	db := testutil.DB(t)
	repo := postgres.NewMemberRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)

	id := uuid.New()
	if err := repo.Create(ctx, &members.Member{
		ID: id, BranchID: branchID, Name: "Old", Email: "old@example.com", Status: members.StatusActive,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Gorm().WithContext(ctx).Unscoped().Delete(&members.Member{}, "id = ?", id).Error
	})

	name := "New"
	status := members.StatusSuspended
	if err := repo.Update(ctx, id, &members.Patch{Name: &name, Status: &status}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "New" || got.Status != members.StatusSuspended {
		t.Fatalf("after update = %+v", got)
	}
	if got.Email != "old@example.com" || got.BranchID != branchID {
		t.Fatalf("unchanged fields modified: %+v", got)
	}
}

func TestMemberRepositoryUpdateNotFound(t *testing.T) {
	db := testutil.DB(t)
	repo := postgres.NewMemberRepository(db)
	ctx := context.Background()

	name := "X"
	err := repo.Update(ctx, uuid.New(), &members.Patch{Name: &name})
	if !errors.Is(err, members.ErrNotFound) {
		t.Fatalf("Update missing = %v, want ErrNotFound", err)
	}
}

func TestMemberRepositoryUpdateDuplicateEmail(t *testing.T) {
	db := testutil.DB(t)
	repo := postgres.NewMemberRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)

	id1 := uuid.New()
	id2 := uuid.New()
	if err := repo.Create(ctx, &members.Member{ID: id1, BranchID: branchID, Name: "One", Email: "one@example.com"}); err != nil {
		t.Fatalf("Create first: %v", err)
	}
	if err := repo.Create(ctx, &members.Member{ID: id2, BranchID: branchID, Name: "Two", Email: "two@example.com"}); err != nil {
		t.Fatalf("Create second: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Gorm().WithContext(ctx).Unscoped().Delete(&members.Member{}, "id IN ?", []uuid.UUID{id1, id2}).Error
	})

	email := "one@example.com"
	err := repo.Update(ctx, id2, &members.Patch{Email: &email})
	if !errors.Is(err, members.ErrDuplicateEmail) {
		t.Fatalf("Update duplicate = %v, want ErrDuplicateEmail", err)
	}
}

func TestMemberRepositoryDelete(t *testing.T) {
	db := testutil.DB(t)
	repo := postgres.NewMemberRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)

	id := uuid.New()
	if err := repo.Create(ctx, &members.Member{ID: id, BranchID: branchID, Name: "Ada", Email: "ada@example.com"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, id); !errors.Is(err, members.ErrNotFound) {
		t.Fatalf("GetByID after delete = %v, want ErrNotFound", err)
	}
	err := repo.Delete(ctx, id)
	if !errors.Is(err, members.ErrNotFound) {
		t.Fatalf("Delete missing = %v, want ErrNotFound", err)
	}
}

func TestMemberRepositoryList(t *testing.T) {
	db := testutil.DB(t)
	repo := postgres.NewMemberRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)

	names := []string{"Zoe", "Ada", "Mia"}
	var ids []uuid.UUID
	for _, n := range names {
		id := uuid.New()
		ids = append(ids, id)
		if err := repo.Create(ctx, &members.Member{
			ID: id, BranchID: branchID, Name: n, Email: n + "@example.com",
		}); err != nil {
			t.Fatalf("Create(%s): %v", n, err)
		}
		t.Cleanup(func() {
			_ = db.Gorm().WithContext(ctx).Unscoped().Delete(&members.Member{}, "id = ?", id).Error
		})
	}

	got, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	byName := map[string]string{}
	order := map[string]int{}
	for i, m := range got {
		byName[m.Name] = m.Name
		order[m.Name] = i
	}
	for _, want := range names {
		if _, ok := byName[want]; !ok {
			t.Fatalf("List missing %q; got %+v", want, got)
		}
	}
	wantOrder := []string{"Ada", "Mia", "Zoe"}
	for i := 1; i < len(wantOrder); i++ {
		if order[wantOrder[i-1]] > order[wantOrder[i]] {
			t.Fatalf("List not ordered: %+v", got)
		}
	}
}

func createTestBranch(t *testing.T, db *postgres.DB) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	repo := postgres.NewBranchRepository(db)
	if err := repo.Create(ctx, &branches.Branch{ID: id, Name: "Test Gym", Address: "1 Gym St"}); err != nil {
		t.Fatalf("create branch: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Gorm().WithContext(ctx).Unscoped().Delete(&branches.Branch{}, "id = ?", id).Error
	})
	return id
}
