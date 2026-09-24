package memberships

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/members"
	"github.com/PandaX185/fitcore/internal/modules/packages"
)

type fakeRepo struct {
	create                   func(ctx context.Context, m *Membership) error
	getByID                  func(ctx context.Context, id uuid.UUID) (*Membership, error)
	listByMember             func(ctx context.Context, memberID uuid.UUID) ([]*Membership, error)
	update                   func(ctx context.Context, id uuid.UUID, patch *Patch) error
	hasActiveByMember        func(ctx context.Context, memberID uuid.UUID) (bool, error)
	findActiveByMemberBranch func(ctx context.Context, memberID, branchID uuid.UUID) (*Membership, error)
}

func (f fakeRepo) Create(ctx context.Context, m *Membership) error {
	if f.create == nil {
		return nil
	}
	return f.create(ctx, m)
}
func (f fakeRepo) GetByID(ctx context.Context, id uuid.UUID) (*Membership, error) {
	return f.getByID(ctx, id)
}
func (f fakeRepo) ListByMember(ctx context.Context, memberID uuid.UUID) ([]*Membership, error) {
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
func (f fakeRepo) HasActiveByMember(ctx context.Context, memberID uuid.UUID) (bool, error) {
	if f.hasActiveByMember == nil {
		return false, nil
	}
	return f.hasActiveByMember(ctx, memberID)
}
func (f fakeRepo) FindActiveByMemberAndBranch(ctx context.Context, memberID, branchID uuid.UUID) (*Membership, error) {
	if f.findActiveByMemberBranch == nil {
		return nil, ErrNotFound
	}
	return f.findActiveByMemberBranch(ctx, memberID, branchID)
}

type fakePackages struct {
	get func(ctx context.Context, id uuid.UUID) (*packages.Package, error)
}

func (f fakePackages) Get(ctx context.Context, id uuid.UUID) (*packages.Package, error) {
	return f.get(ctx, id)
}

type fakeMembers struct {
	get func(ctx context.Context, id uuid.UUID) (*members.Member, error)
}

func (f fakeMembers) Get(ctx context.Context, id uuid.UUID) (*members.Member, error) {
	return f.get(ctx, id)
}

func emptyMembers() fakeMembers {
	return fakeMembers{get: func(_ context.Context, id uuid.UUID) (*members.Member, error) {
		return &members.Member{ID: id}, nil
	}}
}

func emptyPackages(days int) fakePackages {
	return fakePackages{get: func(_ context.Context, id uuid.UUID) (*packages.Package, error) {
		return &packages.Package{ID: id, DurationDays: days}, nil
	}}
}

func TestCreate(t *testing.T) {
	memberID, packageID, branchID := uuid.New(), uuid.New(), uuid.New()
	var created *Membership
	svc := NewService(fakeRepo{
		hasActiveByMember: func(context.Context, uuid.UUID) (bool, error) { return false, nil },
		create: func(_ context.Context, m *Membership) error {
			created = m
			return nil
		},
	}, emptyPackages(30), emptyMembers())

	got, err := svc.Create(context.Background(), memberID, packageID, branchID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID != created.ID || created.MemberID != memberID || created.PackageID != packageID || created.BranchID != branchID {
		t.Fatalf("membership = %+v", created)
	}
	if created.Status != StatusActive {
		t.Fatalf("status = %q, want active", created.Status)
	}
	if want := created.StartsAt.AddDate(0, 0, 30); !created.ExpiresAt.Equal(want) {
		t.Fatalf("expiry = %v, want %v (30d)", created.ExpiresAt, want)
	}
}

func TestCreateInvalid(t *testing.T) {
	svc := NewService(fakeRepo{}, emptyPackages(30), emptyMembers())
	id := uuid.New()
	for _, args := range [][3]uuid.UUID{
		{uuid.Nil, id, id},
		{id, uuid.Nil, id},
		{id, id, uuid.Nil},
	} {
		if _, err := svc.Create(context.Background(), args[0], args[1], args[2]); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Create(%v) = %v, want ErrInvalidInput", args, err)
		}
	}
}

func TestCreateMemberNotFound(t *testing.T) {
	svc := NewService(fakeRepo{}, emptyPackages(30), fakeMembers{get: func(context.Context, uuid.UUID) (*members.Member, error) {
		return nil, members.ErrNotFound
	}})
	if _, err := svc.Create(context.Background(), uuid.New(), uuid.New(), uuid.New()); !errors.Is(err, ErrMemberNotFound) {
		t.Fatalf("Create = %v, want ErrMemberNotFound", err)
	}
}

func TestCreatePackageNotFound(t *testing.T) {
	svc := NewService(fakeRepo{}, fakePackages{get: func(context.Context, uuid.UUID) (*packages.Package, error) {
		return nil, packages.ErrNotFound
	}}, emptyMembers())
	if _, err := svc.Create(context.Background(), uuid.New(), uuid.New(), uuid.New()); !errors.Is(err, ErrPackageNotFound) {
		t.Fatalf("Create = %v, want ErrPackageNotFound", err)
	}
}

func TestCreateDuplicateActive(t *testing.T) {
	svc := NewService(fakeRepo{
		hasActiveByMember: func(context.Context, uuid.UUID) (bool, error) { return true, nil },
	}, emptyPackages(30), emptyMembers())
	if _, err := svc.Create(context.Background(), uuid.New(), uuid.New(), uuid.New()); !errors.Is(err, ErrDuplicateActive) {
		t.Fatalf("Create = %v, want ErrDuplicateActive", err)
	}
}

func TestUpdate(t *testing.T) {
	id := uuid.New()
	start := time.Now().UTC().Add(-5 * 24 * time.Hour)
	svc := NewService(fakeRepo{
		getByID: func(_ context.Context, got uuid.UUID) (*Membership, error) {
			return &Membership{ID: got, StartsAt: start, ExpiresAt: start.Add(30 * 24 * time.Hour), Status: StatusActive}, nil
		},
		update: func(_ context.Context, got uuid.UUID, patch *Patch) error {
			if got != id || patch.Status == nil || *patch.Status != StatusFrozen {
				t.Fatalf("Update(%v, %+v)", got, patch)
			}
			return nil
		},
	}, emptyPackages(30), emptyMembers())
	s := StatusFrozen
	if _, err := svc.Update(context.Background(), id, Patch{Status: &s}); err != nil {
		t.Fatalf("Update: %v", err)
	}
}

func TestUpdateInvalid(t *testing.T) {
	start := time.Now().UTC().Add(-5 * 24 * time.Hour)
	svc := NewService(fakeRepo{
		getByID: func(context.Context, uuid.UUID) (*Membership, error) {
			return &Membership{StartsAt: start, ExpiresAt: start.Add(30 * 24 * time.Hour)}, nil
		},
	}, emptyPackages(30), emptyMembers())
	bad := Status("garbage")
	before := start.Add(-1 * time.Hour)
	if _, err := svc.Update(context.Background(), uuid.Nil, Patch{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Update nil id = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.Update(context.Background(), uuid.New(), Patch{Status: &bad}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Update bad status = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.Update(context.Background(), uuid.New(), Patch{ExpiresAt: &before}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Update early expiry = %v, want ErrInvalidInput", err)
	}
}

func TestFindActiveByMemberAndBranch(t *testing.T) {
	want := &Membership{ID: uuid.New()}
	svc := NewService(fakeRepo{
		findActiveByMemberBranch: func(_ context.Context, m, b uuid.UUID) (*Membership, error) {
			return want, nil
		},
	}, emptyPackages(30), emptyMembers())
	got, err := svc.FindActiveByMemberAndBranch(context.Background(), uuid.New(), uuid.New())
	if err != nil || got != want {
		t.Fatalf("FindActive = %v, %v", got, err)
	}
}

func TestListByMemberInvalid(t *testing.T) {
	svc := NewService(fakeRepo{}, emptyPackages(30), emptyMembers())
	if _, err := svc.ListByMember(context.Background(), uuid.Nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ListByMember nil = %v, want ErrInvalidInput", err)
	}
}
