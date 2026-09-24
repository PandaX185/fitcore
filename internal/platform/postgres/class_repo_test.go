//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/classes"
	"github.com/PandaX185/fitcore/internal/platform/postgres"
)

func TestClassRepositoryCreateAndGet(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewClassRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	if _, err := repo.GetByID(ctx, uuid.New()); !errors.Is(err, classes.ErrNotFound) {
		t.Fatalf("GetByID missing = %v, want ErrNotFound", err)
	}

	id := uuid.New()
	start := time.Now().UTC().Add(24 * time.Hour)
	c := &classes.Class{
		ID: id, BranchID: branchID, Name: "Spin", Capacity: 20,
		StartsAt: start, EndsAt: start.Add(time.Hour), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := repo.Create(ctx, c); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupTable(t, db, "classes", id)

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != id || got.BranchID != branchID || got.Name != "Spin" || got.Capacity != 20 {
		t.Fatalf("GetByID = %+v, want %+v", got, c)
	}
	assertTimeEqual(t, got.StartsAt, start)
	assertTimeEqual(t, got.EndsAt, start.Add(time.Hour))
}

func TestClassRepositoryCreateWithTrainer(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewClassRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	trainerID := createTestTrainer(t, db, branchID)

	id := uuid.New()
	start := time.Now().UTC().Add(24 * time.Hour)
	if err := repo.Create(ctx, &classes.Class{
		ID: id, BranchID: branchID, TrainerID: &trainerID, Name: "Yoga", Capacity: 10,
		StartsAt: start, EndsAt: start.Add(time.Hour),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupTable(t, db, "classes", id)

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.TrainerID == nil || *got.TrainerID != trainerID {
		t.Fatalf("trainer = %v, want %v", got.TrainerID, trainerID)
	}
}

func TestClassRepositoryListFilters(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewClassRepository(db)
	ctx := context.Background()

	branchA := createTestBranch(t, db)
	branchB := createTestBranch(t, db)
	trainerID := createTestTrainer(t, db, branchA)

	idA := uuid.New()
	startA := time.Now().UTC().Add(24 * time.Hour)
	if err := repo.Create(ctx, &classes.Class{ID: idA, BranchID: branchA, Name: "A Class", Capacity: 5, StartsAt: startA, EndsAt: startA.Add(time.Hour)}); err != nil {
		t.Fatalf("Create A: %v", err)
	}
	cleanupTable(t, db, "classes", idA)

	idB := uuid.New()
	startB := time.Now().UTC().Add(48 * time.Hour)
	if err := repo.Create(ctx, &classes.Class{ID: idB, BranchID: branchB, Name: "B Class", Capacity: 5, StartsAt: startB, EndsAt: startB.Add(time.Hour)}); err != nil {
		t.Fatalf("Create B: %v", err)
	}
	cleanupTable(t, db, "classes", idB)

	idT := uuid.New()
	startT := time.Now().UTC().Add(72 * time.Hour)
	if err := repo.Create(ctx, &classes.Class{ID: idT, BranchID: branchA, TrainerID: &trainerID, Name: "T Class", Capacity: 5, StartsAt: startT, EndsAt: startT.Add(time.Hour)}); err != nil {
		t.Fatalf("Create T: %v", err)
	}
	cleanupTable(t, db, "classes", idT)

	byBranch, err := repo.List(ctx, &branchA, nil)
	if err != nil {
		t.Fatalf("List(branchA): %v", err)
	}
	if len(byBranch) != 2 {
		t.Fatalf("List(branchA) = %d, want 2", len(byBranch))
	}
	for _, c := range byBranch {
		if c.BranchID != branchA {
			t.Fatalf("branch leak: %+v", c)
		}
	}

	byTrainer, err := repo.List(ctx, nil, &trainerID)
	if err != nil {
		t.Fatalf("List(trainer): %v", err)
	}
	if len(byTrainer) != 1 || byTrainer[0].ID != idT {
		t.Fatalf("List(trainer) = %+v", byTrainer)
	}

	all, err := repo.List(ctx, nil, nil)
	if err != nil || len(all) < 3 {
		t.Fatalf("List(all) = %d, %v; want >= 3", len(all), err)
	}
}

func TestClassRepositoryUpdateAndDelete(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewClassRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	id := uuid.New()
	start := time.Now().UTC().Add(24 * time.Hour)
	if err := repo.Create(ctx, &classes.Class{ID: id, BranchID: branchID, Name: "Old", Capacity: 5, StartsAt: start, EndsAt: start.Add(time.Hour)}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupTable(t, db, "classes", id)

	name := "New"
	capacity := 25
	if err := repo.Update(ctx, id, &classes.Patch{Name: &name, Capacity: &capacity}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "New" || got.Capacity != 25 {
		t.Fatalf("after update = %+v", got)
	}

	if err := repo.Update(ctx, uuid.New(), &classes.Patch{Name: &name}); !errors.Is(err, classes.ErrNotFound) {
		t.Fatalf("Update missing = %v, want ErrNotFound", err)
	}

	if err := repo.Delete(ctx, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, id); !errors.Is(err, classes.ErrNotFound) {
		t.Fatalf("GetByID after delete = %v, want ErrNotFound", err)
	}
	if err := repo.Delete(ctx, id); !errors.Is(err, classes.ErrNotFound) {
		t.Fatalf("Delete missing = %v, want ErrNotFound", err)
	}
}
