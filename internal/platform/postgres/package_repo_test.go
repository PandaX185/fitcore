//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/packages"
	"github.com/PandaX185/fitcore/internal/platform/postgres"
)

func TestPackageRepositoryCreateAndGet(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewPackageRepository(db)
	ctx := context.Background()

	if _, err := repo.GetByID(ctx, uuid.New()); !errors.Is(err, packages.ErrNotFound) {
		t.Fatalf("GetByID missing = %v, want ErrNotFound", err)
	}

	id := uuid.New()
	p := &packages.Package{
		ID: id, Name: "Monthly " + uuid.NewString()[:6], DurationDays: 30,
		PriceCents: 25000, Currency: "BHD", Active: true,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupTable(t, db, "membership_packages", id)

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != id || got.Name != p.Name || got.DurationDays != 30 || got.PriceCents != 25000 || got.Currency != "BHD" || !got.Active {
		t.Fatalf("GetByID = %+v, want %+v", got, p)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("timestamps not populated: %+v", got)
	}
}

func TestPackageRepositoryList(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewPackageRepository(db)
	ctx := context.Background()

	names := []string{"Zulu Pkg", "Alpha Pkg", "Mike Pkg"}
	var ids []uuid.UUID
	for _, n := range names {
		id := uuid.New()
		ids = append(ids, id)
		if err := repo.Create(ctx, &packages.Package{ID: id, Name: n + " " + uuid.NewString()[:4], DurationDays: 30, Currency: "USD"}); err != nil {
			t.Fatalf("Create(%s): %v", n, err)
		}
		cleanupTable(t, db, "membership_packages", id)
	}

	got, err := repo.List(ctx, &packages.ListQuery{Limit: 100})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) < len(names) {
		t.Fatalf("List returned %d, want >= %d", len(got), len(names))
	}
}

func TestPackageRepositoryListPaginates(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewPackageRepository(db)
	ctx := context.Background()

	tag := uuid.NewString()[:8]
	names := []string{"Plumb " + tag, "Quill " + tag, "Ridge " + tag, "Slate " + tag}
	for _, n := range names {
		id := uuid.New()
		if err := repo.Create(ctx, &packages.Package{ID: id, Name: n, DurationDays: 30, Currency: "USD"}); err != nil {
			t.Fatalf("Create(%s): %v", n, err)
		}
		cleanupTable(t, db, "membership_packages", id)
	}

	var collected []string
	var afterName string
	var afterID uuid.UUID
	for {
		res, err := repo.List(ctx, &packages.ListQuery{Limit: 2, AfterName: afterName, AfterID: afterID})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(res) == 0 {
			break
		}
		for _, p := range res {
			collected = append(collected, p.Name)
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

func TestPackageRepositoryUpdate(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewPackageRepository(db)
	ctx := context.Background()

	id := uuid.New()
	if err := repo.Create(ctx, &packages.Package{ID: id, Name: "Old " + uuid.NewString()[:4], DurationDays: 30, Currency: "USD"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupTable(t, db, "membership_packages", id)

	name := "Renamed"
	price := int64(50000)
	active := false
	if err := repo.Update(ctx, id, &packages.Patch{Name: &name, PriceCents: &price, Active: &active}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Renamed" || got.PriceCents != 50000 || got.Active {
		t.Fatalf("after update = %+v", got)
	}
	if got.DurationDays != 30 {
		t.Fatalf("unchanged duration modified: %+v", got)
	}
}

func TestPackageRepositoryUpdateNotFoundAndDuplicate(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewPackageRepository(db)
	ctx := context.Background()

	name := "X"
	if err := repo.Update(ctx, uuid.New(), &packages.Patch{Name: &name}); !errors.Is(err, packages.ErrNotFound) {
		t.Fatalf("Update missing = %v, want ErrNotFound", err)
	}

	first := uuid.New()
	second := uuid.New()
	shared := "Dup " + uuid.NewString()[:4]
	if err := repo.Create(ctx, &packages.Package{ID: first, Name: shared, DurationDays: 30, Currency: "USD"}); err != nil {
		t.Fatalf("Create first: %v", err)
	}
	if err := repo.Create(ctx, &packages.Package{ID: second, Name: shared, DurationDays: 30, Currency: "USD"}); !errors.Is(err, packages.ErrDuplicateName) {
		t.Fatalf("Create dup = %v, want ErrDuplicateName", err)
	}
	cleanupTable(t, db, "membership_packages", first)
	cleanupTable(t, db, "membership_packages", second)
}
