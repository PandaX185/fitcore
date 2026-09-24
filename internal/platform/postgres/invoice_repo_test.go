//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/billing"
	"github.com/PandaX185/fitcore/internal/modules/memberships"
	"github.com/PandaX185/fitcore/internal/platform/postgres"
)

func TestInvoiceRepositoryCreateAndGet(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewInvoiceRepository(db)
	ctx := context.Background()

	if _, err := repo.GetByID(ctx, uuid.New()); !errors.Is(err, billing.ErrNotFound) {
		t.Fatalf("GetByID missing = %v, want ErrNotFound", err)
	}

	branchID := createTestBranch(t, db)
	memberID := createTestMember(t, db, branchID)
	packageID := createTestPackage(t, db)
	membershipID := createTestMembership(t, db, memberID, packageID, branchID, memberships.StatusActive)

	id := uuid.New()
	now := time.Now().UTC()
	inv := &billing.Invoice{
		ID: id, MemberID: memberID, MembershipID: membershipID, AmountCents: 25000, Currency: "BHD",
		Status: billing.StatusPending, DueAt: now.AddDate(0, 0, 7), CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Create(ctx, inv); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupTable(t, db, "invoices", id)

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != id || got.MemberID != memberID || got.MembershipID != membershipID || got.AmountCents != 25000 || got.Currency != "BHD" {
		t.Fatalf("GetByID = %+v, want %+v", got, inv)
	}
	if got.Status != billing.StatusPending || got.PaidAt != nil {
		t.Fatalf("status/paid = %q/%v, want pending/nil", got.Status, got.PaidAt)
	}
}

func TestInvoiceRepositoryUpdatePaidStamps(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewInvoiceRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	memberID := createTestMember(t, db, branchID)
	packageID := createTestPackage(t, db)
	membershipID := createTestMembership(t, db, memberID, packageID, branchID, memberships.StatusActive)

	id := uuid.New()
	now := time.Now().UTC()
	if err := repo.Create(ctx, &billing.Invoice{
		ID: id, MemberID: memberID, MembershipID: membershipID, AmountCents: 100, Currency: "USD",
		Status: billing.StatusPending, DueAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupTable(t, db, "invoices", id)

	paid := billing.StatusPaid
	if err := repo.Update(ctx, id, &billing.Patch{Status: &paid}); err != nil {
		t.Fatalf("Update paid: %v", err)
	}
	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != billing.StatusPaid || got.PaidAt == nil {
		t.Fatalf("paid = %q, paid_at=%v; want paid + stamped", got.Status, got.PaidAt)
	}

	status := billing.StatusFailed
	if err := repo.Update(ctx, id, &billing.Patch{Status: &status}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	got, err = repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.PaidAt != nil {
		t.Fatalf("paid_at not cleared after leaving paid: %+v", got)
	}

	if err := repo.Update(ctx, uuid.New(), &billing.Patch{Status: &paid}); !errors.Is(err, billing.ErrNotFound) {
		t.Fatalf("Update missing = %v, want ErrNotFound", err)
	}
}

func TestInvoiceRepositoryListByMember(t *testing.T) {
	db := testutilDB(t)
	repo := postgres.NewInvoiceRepository(db)
	ctx := context.Background()

	branchID := createTestBranch(t, db)
	memberID := createTestMember(t, db, branchID)
	packageID := createTestPackage(t, db)
	membershipID := createTestMembership(t, db, memberID, packageID, branchID, memberships.StatusActive)

	ids := make([]uuid.UUID, 0, 2)
	for i := 0; i < 2; i++ {
		id := uuid.New()
		now := time.Now().UTC().Add(time.Duration(-i) * time.Hour)
		if err := repo.Create(ctx, &billing.Invoice{
			ID: id, MemberID: memberID, MembershipID: membershipID, AmountCents: 100, Currency: "USD",
			Status: billing.StatusPending, DueAt: now.AddDate(0, 0, 7), CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
		ids = append(ids, id)
	}
	for _, id := range ids {
		cleanupTable(t, db, "invoices", id)
	}

	got, err := repo.ListByMember(ctx, memberID)
	if err != nil {
		t.Fatalf("ListByMember: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListByMember = %d, want 2", len(got))
	}
	if got[0].CreatedAt.Before(got[1].CreatedAt) {
		t.Fatalf("expected newest-first, got %v then %v", got[0].CreatedAt, got[1].CreatedAt)
	}
}
