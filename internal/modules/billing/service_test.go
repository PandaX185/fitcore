package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/members"
	"github.com/PandaX185/fitcore/internal/modules/memberships"
)

type fakeRepo struct {
	create       func(ctx context.Context, inv *Invoice) error
	getByID      func(ctx context.Context, id uuid.UUID) (*Invoice, error)
	listByMember func(ctx context.Context, memberID uuid.UUID) ([]*Invoice, error)
	update       func(ctx context.Context, id uuid.UUID, patch *Patch) error
}

func (f fakeRepo) Create(ctx context.Context, inv *Invoice) error {
	if f.create == nil {
		return nil
	}
	return f.create(ctx, inv)
}
func (f fakeRepo) GetByID(ctx context.Context, id uuid.UUID) (*Invoice, error) {
	return f.getByID(ctx, id)
}
func (f fakeRepo) ListByMember(ctx context.Context, memberID uuid.UUID) ([]*Invoice, error) {
	if f.listByMember == nil {
		return nil, nil
	}
	return f.listByMember(ctx, memberID)
}
func (f fakeRepo) Update(ctx context.Context, id uuid.UUID, patch *Patch) error {
	if f.update == nil {
		return nil
	}
	return f.update(ctx, id, patch)
}

type fakeMembers struct {
	get func(ctx context.Context, id uuid.UUID) (*members.Member, error)
}

func (f fakeMembers) Get(ctx context.Context, id uuid.UUID) (*members.Member, error) {
	return f.get(ctx, id)
}

type fakeMemberships struct {
	get func(ctx context.Context, id uuid.UUID) (*memberships.Membership, error)
}

func (f fakeMemberships) Get(ctx context.Context, id uuid.UUID) (*memberships.Membership, error) {
	return f.get(ctx, id)
}

func existingMember() fakeMembers {
	return fakeMembers{get: func(_ context.Context, id uuid.UUID) (*members.Member, error) {
		return &members.Member{ID: id}, nil
	}}
}

func existingMembership() fakeMemberships {
	return fakeMemberships{get: func(_ context.Context, id uuid.UUID) (*memberships.Membership, error) {
		return &memberships.Membership{ID: id}, nil
	}}
}

func TestCreate(t *testing.T) {
	memberID, membershipID := uuid.New(), uuid.New()
	due := time.Now().UTC().Add(7 * 24 * time.Hour)
	var created *Invoice
	svc := NewService(fakeRepo{create: func(_ context.Context, inv *Invoice) error {
		created = inv
		return nil
	}}, existingMember(), existingMembership())

	got, err := svc.Create(context.Background(), memberID, membershipID, 12500, "  bhd  ", due)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID != created.ID || created.MemberID != memberID || created.MembershipID != membershipID {
		t.Fatalf("invoice = %+v", created)
	}
	if created.Status != StatusPending {
		t.Fatalf("status = %q, want pending", created.Status)
	}
	if created.Currency != "BHD" {
		t.Fatalf("currency = %q, want normalized BHD", created.Currency)
	}
	if created.AmountCents != 12500 || !created.DueAt.Equal(due) {
		t.Fatalf("invoice = %+v", created)
	}
}

func TestCreateInvalid(t *testing.T) {
	due := time.Now().UTC().Add(7 * 24 * time.Hour)
	svc := NewService(fakeRepo{}, existingMember(), existingMembership())
	id := uuid.New()
	tests := []struct {
		name        string
		memberID    uuid.UUID
		membership  uuid.UUID
		amountCents int64
		currency    string
		dueAt       time.Time
	}{
		{"nil member", uuid.Nil, id, 100, "USD", due},
		{"nil membership", id, uuid.Nil, 100, "USD", due},
		{"neg amount", id, id, -1, "USD", due},
		{"zero due", id, id, 100, "USD", time.Time{}},
		{"short currency", id, id, 100, "US", due},
		{"long currency", id, id, 100, "USDD", due},
		{"alpha-digit currency", id, id, 100, "1SD", due},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), tt.memberID, tt.membership, tt.amountCents, tt.currency, tt.dueAt)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Create = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestCreateMemberNotFound(t *testing.T) {
	svc := NewService(fakeRepo{}, fakeMembers{get: func(context.Context, uuid.UUID) (*members.Member, error) {
		return nil, members.ErrNotFound
	}}, existingMembership())
	if _, err := svc.Create(context.Background(), uuid.New(), uuid.New(), 100, "USD", time.Now().Add(time.Hour)); !errors.Is(err, ErrMemberNotFound) {
		t.Fatalf("Create = %v, want ErrMemberNotFound", err)
	}
}

func TestCreateMembershipNotFound(t *testing.T) {
	svc := NewService(fakeRepo{}, existingMember(), fakeMemberships{get: func(context.Context, uuid.UUID) (*memberships.Membership, error) {
		return nil, memberships.ErrNotFound
	}})
	if _, err := svc.Create(context.Background(), uuid.New(), uuid.New(), 100, "USD", time.Now().Add(time.Hour)); !errors.Is(err, ErrMembershipNotFound) {
		t.Fatalf("Create = %v, want ErrMembershipNotFound", err)
	}
}

func TestUpdate(t *testing.T) {
	id := uuid.New()
	paid := InvoiceStatus(StatusPaid)
	var patches []Patch
	svc := NewService(fakeRepo{
		getByID: func(context.Context, uuid.UUID) (*Invoice, error) {
			return &Invoice{ID: id, Status: StatusPending}, nil
		},
		update: func(_ context.Context, got uuid.UUID, patch *Patch) error {
			patches = append(patches, *patch)
			return nil
		},
	}, existingMember(), existingMembership())
	if _, err := svc.Update(context.Background(), id, Patch{Status: &paid}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(patches) != 1 || patches[0].Status == nil || *patches[0].Status != StatusPaid {
		t.Fatalf("patches = %+v", patches)
	}
}

func TestUpdatePaidCannotRevert(t *testing.T) {
	id := uuid.New()
	pending := InvoiceStatus(StatusPending)
	svc := NewService(fakeRepo{
		getByID: func(_ context.Context, got uuid.UUID) (*Invoice, error) {
			return &Invoice{ID: got, Status: StatusPaid}, nil
		},
	}, existingMember(), existingMembership())
	if _, err := svc.Update(context.Background(), id, Patch{Status: &pending}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Update paid->pending = %v, want ErrInvalidInput", err)
	}
}

func TestUpdateInvalid(t *testing.T) {
	bad := InvoiceStatus("garbage")
	zero := time.Time{}
	svc := NewService(fakeRepo{
		getByID: func(context.Context, uuid.UUID) (*Invoice, error) {
			return &Invoice{Status: StatusPending}, nil
		},
	}, existingMember(), existingMembership())
	if _, err := svc.Update(context.Background(), uuid.Nil, Patch{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Update nil id = %v", err)
	}
	if _, err := svc.Update(context.Background(), uuid.New(), Patch{Status: &bad}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Update bad status = %v", err)
	}
	if _, err := svc.Update(context.Background(), uuid.New(), Patch{DueAt: &zero}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Update zero due = %v", err)
	}
}

func TestListByMember(t *testing.T) {
	memberID := uuid.New()
	svc := NewService(fakeRepo{listByMember: func(_ context.Context, got uuid.UUID) ([]*Invoice, error) {
		if got != memberID {
			t.Fatalf("ListByMember(%v)", got)
		}
		return []*Invoice{{ID: uuid.New()}}, nil
	}}, existingMember(), existingMembership())
	got, err := svc.ListByMember(context.Background(), memberID)
	if err != nil || len(got) != 1 {
		t.Fatalf("ListByMember = %v, %v", got, err)
	}
	if _, err := svc.ListByMember(context.Background(), uuid.Nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ListByMember nil = %v", err)
	}
}
