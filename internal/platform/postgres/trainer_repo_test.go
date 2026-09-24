//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/trainers"
	"github.com/PandaX185/fitcore/internal/platform/postgres"
)

func TestTrainerRepositoryCreateAndGet(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewTrainerRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	if _, err := repo.GetByID(ctx, uuid.New()); !errors.Is(err, trainers.ErrNotFound) {
		t.Fatalf("GetByID missing = %v, want ErrNotFound", err)
	}

	id := uuid.New()
	tr := &trainers.Trainer{
		ID: id, BranchID: branchID, Name: "Coach One", Email: uuid.NewString()[:8] + "@example.com",
		Phone: "555", Active: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := repo.Create(ctx, tr); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupTable(t, db, "trainers", id)

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != id || got.BranchID != branchID || got.Name != tr.Name || got.Email != tr.Email || got.Phone != tr.Phone || !got.Active {
		t.Fatalf("GetByID = %+v, want %+v", got, tr)
	}
}

func TestTrainerRepositoryDuplicateEmail(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewTrainerRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	email := uuid.NewString()[:8] + "@example.com"

	id1 := uuid.New()
	if err := repo.Create(ctx, &trainers.Trainer{ID: id1, BranchID: branchID, Name: "One", Email: email}); err != nil {
		t.Fatalf("Create first: %v", err)
	}
	cleanupTable(t, db, "trainers", id1)

	err := repo.Create(ctx, &trainers.Trainer{ID: uuid.New(), BranchID: branchID, Name: "Two", Email: email})
	if !errors.Is(err, trainers.ErrDuplicateEmail) {
		t.Fatalf("Create dup = %v, want ErrDuplicateEmail", err)
	}
}

func TestTrainerRepositoryUpdate(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewTrainerRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	id := uuid.New()
	if err := repo.Create(ctx, &trainers.Trainer{ID: id, BranchID: branchID, Name: "Old", Email: uuid.NewString()[:8] + "@example.com"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupTable(t, db, "trainers", id)

	name := "New"
	active := false
	if err := repo.Update(ctx, id, &trainers.Patch{Name: &name, Active: &active}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "New" || got.Active {
		t.Fatalf("after update = %+v", got)
	}

	if err := repo.Update(ctx, uuid.New(), &trainers.Patch{Name: &name}); !errors.Is(err, trainers.ErrNotFound) {
		t.Fatalf("Update missing = %v, want ErrNotFound", err)
	}
}

func TestTrainerRepositoryListByBranch(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewTrainerRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	id := uuid.New()
	if err := repo.Create(ctx, &trainers.Trainer{ID: id, BranchID: branchID, Name: "Branch Coach", Email: uuid.NewString()[:8] + "@example.com"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupTable(t, db, "trainers", id)

	got, err := repo.ListByBranch(ctx, branchID)
	if err != nil {
		t.Fatalf("ListByBranch: %v", err)
	}
	if len(got) < 1 {
		t.Fatalf("ListByBranch returned %d, want >= 1", len(got))
	}
}
