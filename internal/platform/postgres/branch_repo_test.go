//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/branches"
	"github.com/PandaX185/fitcore/internal/platform/postgres"
	"github.com/PandaX185/fitcore/internal/testutil"
)

func TestBranchRepositoryGetByID(t *testing.T) {
	db := testutil.DB(t)
	repo := postgres.NewBranchRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	if !errors.Is(err, branches.ErrNotFound) {
		t.Fatalf("GetByID missing = %v, want ErrNotFound", err)
	}

	id := uuid.New()
	b := &branches.Branch{ID: id, Name: "Test Gym", Address: "1 Gym St", Latitude: 51.5, Longitude: -0.1}
	if err := repo.Create(ctx, b); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Gorm().WithContext(ctx).Unscoped().Delete(&branches.Branch{}, "id = ?", id).Error
	})

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != id || got.Name != b.Name || got.Address != b.Address {
		t.Fatalf("GetByID = %+v, want %+v", got, b)
	}
	if got.Latitude != b.Latitude || got.Longitude != b.Longitude {
		t.Fatalf("lat/lon = %v/%v, want %v/%v", got.Latitude, got.Longitude, b.Latitude, b.Longitude)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("timestamps not populated: %+v", got)
	}
}

func TestBranchRepositoryListSearchAndPaginate(t *testing.T) {
	db := testutil.DB(t)
	repo := postgres.NewBranchRepository(db)
	ctx := context.Background()

	names := []string{"Alpha", "Bravo", "Charlie", "Echo", "Fort 100% Grit", "Foxtrot"}
	var ids []uuid.UUID
	for _, n := range names {
		id := uuid.New()
		ids = append(ids, id)
		if err := repo.Create(ctx, &branches.Branch{ID: id, Name: n, Address: n + " Ave"}); err != nil {
			t.Fatalf("Create(%s): %v", n, err)
		}
		t.Cleanup(func() {
			_ = db.Gorm().WithContext(ctx).Unscoped().Delete(&branches.Branch{}, "id = ?", id).Error
		})
	}

	var collected []string
	var afterName string
	var afterID uuid.UUID
	for {
		res, err := repo.List(ctx, &branches.ListQuery{Limit: 2, AfterName: afterName, AfterID: afterID})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(res) == 0 {
			break
		}
		for _, b := range res {
			collected = append(collected, b.Name)
		}
		afterName = res[len(res)-1].Name
		afterID = res[len(res)-1].ID
		if len(res) < 2 {
			break
		}
	}

	if len(collected) != len(names) {
		t.Fatalf("collected %d rows, want %d: %v", len(collected), len(names), collected)
	}
	for i, n := range collected {
		if n != names[i] {
			t.Fatalf("row %d = %q, want %q (not ordered by name)", i, n, names[i])
		}
	}
}

func TestBranchRepositoryListSearch(t *testing.T) {
	db := testutil.DB(t)
	repo := postgres.NewBranchRepository(db)
	ctx := context.Background()

	names := []string{"Alpha", "Bravo", "Fort 100% Grit"}
	var ids []uuid.UUID
	for _, n := range names {
		id := uuid.New()
		ids = append(ids, id)
		if err := repo.Create(ctx, &branches.Branch{ID: id, Name: n, Address: "1 " + n + " St"}); err != nil {
			t.Fatalf("Create(%s): %v", n, err)
		}
		t.Cleanup(func() {
			_ = db.Gorm().WithContext(ctx).Unscoped().Delete(&branches.Branch{}, "id = ?", id).Error
		})
	}

	tests := []struct {
		query string
		want  int
	}{
		{query: "bra", want: 1}, // case-insensitive name substring
		{query: "BRAVO", want: 1},
		{query: "bravo st", want: 1}, // address substring ("1 Bravo St")
		{query: "100%", want: 1},     // escaped wildcard matches literally
		{query: "zzz", want: 0},      // no match
	}
	for _, tt := range tests {
		res, err := repo.List(ctx, &branches.ListQuery{Query: tt.query, Limit: 100, AfterName: "", AfterID: uuid.Nil})
		if err != nil {
			t.Fatalf("List(%q): %v", tt.query, err)
		}
		if len(res) != tt.want {
			t.Fatalf("List(%q) = %d rows, want %d", tt.query, len(res), tt.want)
		}
	}
}

func TestBranchRepositoryUpdate(t *testing.T) {
	db := testutil.DB(t)
	repo := postgres.NewBranchRepository(db)
	ctx := context.Background()

	created := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	id := uuid.New()
	if err := repo.Create(ctx, &branches.Branch{
		ID: id, Name: "Old", Address: "1 St", Latitude: 10, Longitude: 20,
		CreatedAt: created, UpdatedAt: created,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Gorm().WithContext(ctx).Unscoped().Delete(&branches.Branch{}, "id = ?", id).Error
	})

	name := "New"
	if err := repo.Update(ctx, id, &branches.Patch{Name: &name}); err != nil {
		t.Fatalf("Update name: %v", err)
	}

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "New" {
		t.Fatalf("name = %q, want %q", got.Name, "New")
	}
	if got.Address != "1 St" || got.Latitude != 10 || got.Longitude != 20 {
		t.Fatalf("unchanged fields modified: %+v", got)
	}
	if !got.CreatedAt.Equal(created) {
		t.Fatalf("createdAt = %v, want %v", got.CreatedAt, created)
	}
	if !got.UpdatedAt.After(created) {
		t.Fatalf("updatedAt %v not after createdAt %v", got.UpdatedAt, created)
	}

	lat, lon := 51.5, -0.1
	if err := repo.Update(ctx, id, &branches.Patch{Latitude: &lat, Longitude: &lon}); err != nil {
		t.Fatalf("Update coords: %v", err)
	}
	got, err = repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Latitude != lat || got.Longitude != lon {
		t.Fatalf("coords = %v/%v, want %v/%v", got.Latitude, got.Longitude, lat, lon)
	}
}

func TestBranchRepositoryUpdateNotFound(t *testing.T) {
	db := testutil.DB(t)
	repo := postgres.NewBranchRepository(db)
	ctx := context.Background()

	name := "X"
	err := repo.Update(ctx, uuid.New(), &branches.Patch{Name: &name})
	if !errors.Is(err, branches.ErrNotFound) {
		t.Fatalf("Update missing = %v, want ErrNotFound", err)
	}
}
