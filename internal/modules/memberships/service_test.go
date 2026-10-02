package memberships

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/branches"
	"github.com/PandaX185/fitcore/internal/modules/members"
	"github.com/PandaX185/fitcore/internal/modules/packages"
	"github.com/PandaX185/fitcore/internal/paging"
)

type fakeRepo struct {
	create                   func(ctx context.Context, m *Membership) error
	getByID                  func(ctx context.Context, id uuid.UUID) (*Membership, error)
	listByMember             func(ctx context.Context, q *MemberListQuery) ([]*Membership, error)
	update                   func(ctx context.Context, id uuid.UUID, patch *Patch) error
	updateStatus             func(ctx context.Context, id uuid.UUID, expected Status, patch *Patch) error
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
func (f fakeRepo) ListByMember(ctx context.Context, q *MemberListQuery) ([]*Membership, error) {
	if f.listByMember == nil {
		return nil, nil
	}
	return f.listByMember(ctx, q)
}
func (f fakeRepo) Update(ctx context.Context, id uuid.UUID, patch *Patch) error {
	if f.update == nil {
		return nil
	}
	return f.update(ctx, id, patch)
}

// UpdateStatus defaults to the plain update so pre-existing tests exercise
// the same assertions through the conditional path.
func (f fakeRepo) UpdateStatus(ctx context.Context, id uuid.UUID, expected Status, patch *Patch) error {
	if f.updateStatus == nil {
		return f.Update(ctx, id, patch)
	}
	return f.updateStatus(ctx, id, expected, patch)
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

type fakeBranches struct {
	get func(ctx context.Context, id uuid.UUID) (*branches.Branch, error)
}

func (f fakeBranches) Get(ctx context.Context, id uuid.UUID) (*branches.Branch, error) {
	return f.get(ctx, id)
}

type stubTx struct{}

func (stubTx) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func emptyMembers() fakeMembers {
	return fakeMembers{get: func(_ context.Context, id uuid.UUID) (*members.Member, error) {
		return &members.Member{ID: id}, nil
	}}
}

func emptyBranches() fakeBranches {
	return fakeBranches{get: func(_ context.Context, id uuid.UUID) (*branches.Branch, error) {
		return &branches.Branch{ID: id}, nil
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
	}, emptyPackages(30), emptyMembers(), emptyBranches(), stubTx{})

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
	svc := NewService(fakeRepo{}, emptyPackages(30), emptyMembers(), emptyBranches(), stubTx{})
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
	}}, emptyBranches(), stubTx{})
	if _, err := svc.Create(context.Background(), uuid.New(), uuid.New(), uuid.New()); !errors.Is(err, ErrMemberNotFound) {
		t.Fatalf("Create = %v, want ErrMemberNotFound", err)
	}
}

func TestCreatePackageNotFound(t *testing.T) {
	svc := NewService(fakeRepo{}, fakePackages{get: func(context.Context, uuid.UUID) (*packages.Package, error) {
		return nil, packages.ErrNotFound
	}}, emptyMembers(), emptyBranches(), stubTx{})
	if _, err := svc.Create(context.Background(), uuid.New(), uuid.New(), uuid.New()); !errors.Is(err, ErrPackageNotFound) {
		t.Fatalf("Create = %v, want ErrPackageNotFound", err)
	}
}

func TestCreateDuplicateActive(t *testing.T) {
	svc := NewService(fakeRepo{
		hasActiveByMember: func(context.Context, uuid.UUID) (bool, error) { return true, nil },
	}, emptyPackages(30), emptyMembers(), emptyBranches(), stubTx{})
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
	}, emptyPackages(30), emptyMembers(), emptyBranches(), stubTx{})
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
	}, emptyPackages(30), emptyMembers(), emptyBranches(), stubTx{})
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
	}, emptyPackages(30), emptyMembers(), emptyBranches(), stubTx{})
	got, err := svc.FindActiveByMemberAndBranch(context.Background(), uuid.New(), uuid.New())
	if err != nil || got != want {
		t.Fatalf("FindActive = %v, %v", got, err)
	}
}

func TestListByMemberInvalid(t *testing.T) {
	svc := NewService(fakeRepo{}, emptyPackages(30), emptyMembers(), emptyBranches(), stubTx{})
	if _, err := svc.ListByMember(context.Background(), uuid.Nil, MemberListParams{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ListByMember nil = %v, want ErrInvalidInput", err)
	}
}

func TestListByMemberFirstPage(t *testing.T) {
	memberID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)
	svc := NewService(fakeRepo{
		listByMember: func(_ context.Context, q *MemberListQuery) ([]*Membership, error) {
			if q.MemberID != memberID {
				t.Fatalf("MemberID = %v, want %v", q.MemberID, memberID)
			}
			if q.Limit != paging.DefaultLimit+1 {
				t.Fatalf("Limit = %d, want %d", q.Limit, paging.DefaultLimit+1)
			}
			if q.AfterStartsAt != "" || q.AfterID != uuid.Nil {
				t.Fatalf("cursor start = %q/%v, want empty", q.AfterStartsAt, q.AfterID)
			}
			items := []*Membership{}
			for i := 0; i < paging.DefaultLimit+1; i++ {
				items = append(items, &Membership{ID: uuid.New(), StartsAt: now})
			}
			return items, nil
		},
	}, emptyPackages(30), emptyMembers(), emptyBranches(), stubTx{})

	res, err := svc.ListByMember(context.Background(), memberID, MemberListParams{})
	if err != nil {
		t.Fatalf("ListByMember: unexpected error %v", err)
	}
	if len(res.Items) != paging.DefaultLimit {
		t.Fatalf("items = %d, want %d", len(res.Items), paging.DefaultLimit)
	}
	if res.NextCursor == "" {
		t.Fatal("NextCursor empty, want a value when a page is full")
	}
	cur, err := paging.DecodeCursor(res.NextCursor)
	if err != nil {
		t.Fatalf("decode NextCursor: %v", err)
	}
	if cur.Key != now.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("cursor key = %q, want %q", cur.Key, now.UTC().Format(time.RFC3339Nano))
	}
}

func TestListByMemberLastPage(t *testing.T) {
	svc := NewService(fakeRepo{
		listByMember: func(context.Context, *MemberListQuery) ([]*Membership, error) {
			return []*Membership{{ID: uuid.New()}}, nil
		},
	}, emptyPackages(30), emptyMembers(), emptyBranches(), stubTx{})

	res, err := svc.ListByMember(context.Background(), uuid.New(), MemberListParams{Limit: 20})
	if err != nil {
		t.Fatalf("ListByMember: unexpected error %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(res.Items))
	}
	if res.NextCursor != "" {
		t.Fatalf("NextCursor = %q, want empty on the last page", res.NextCursor)
	}
}

func TestListByMemberClampsLimit(t *testing.T) {
	for _, tt := range []struct {
		in   int
		want int
	}{
		{in: 0, want: paging.DefaultLimit},
		{in: -5, want: paging.DefaultLimit},
		{in: 1000, want: paging.MaxLimit},
	} {
		t.Run("", func(t *testing.T) {
			svc := NewService(fakeRepo{
				listByMember: func(_ context.Context, q *MemberListQuery) ([]*Membership, error) {
					if q.Limit != tt.want+1 {
						t.Fatalf("repo Limit = %d, want %d", q.Limit, tt.want+1)
					}
					return nil, nil
				},
			}, emptyPackages(30), emptyMembers(), emptyBranches(), stubTx{})
			if _, err := svc.ListByMember(context.Background(), uuid.New(), MemberListParams{Limit: tt.in}); err != nil {
				t.Fatalf("ListByMember: unexpected error %v", err)
			}
		})
	}
}

func TestListByMemberCursorRoundTrip(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Microsecond)
	id := uuid.New()
	cursor := paging.Cursor{Key: at.Format(time.RFC3339Nano), ID: id}.Encode()

	svc := NewService(fakeRepo{
		listByMember: func(_ context.Context, q *MemberListQuery) ([]*Membership, error) {
			if q.AfterStartsAt != at.Format(time.RFC3339Nano) {
				t.Fatalf("AfterStartsAt = %q, want %q", q.AfterStartsAt, at.Format(time.RFC3339Nano))
			}
			if q.AfterID != id {
				t.Fatalf("AfterID = %v, want %v", q.AfterID, id)
			}
			return nil, nil
		},
	}, emptyPackages(30), emptyMembers(), emptyBranches(), stubTx{})

	if _, err := svc.ListByMember(context.Background(), uuid.New(), MemberListParams{Cursor: cursor}); err != nil {
		t.Fatalf("ListByMember: unexpected error %v", err)
	}
}

func TestListByMemberRejectsBadCursor(t *testing.T) {
	for _, cur := range []string{"%%%", "not-base64-!", "eyJuYW1lIjoibiJ9"} {
		t.Run(cur, func(t *testing.T) {
			svc := NewService(fakeRepo{
				listByMember: func(context.Context, *MemberListQuery) ([]*Membership, error) {
					t.Fatal("repo must not be called for a bad cursor")
					return nil, nil
				},
			}, emptyPackages(30), emptyMembers(), emptyBranches(), stubTx{})
			_, err := svc.ListByMember(context.Background(), uuid.New(), MemberListParams{Cursor: cur})
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("ListByMember error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestCreateBranchNotFound(t *testing.T) {
	svc := NewService(fakeRepo{}, emptyPackages(30), emptyMembers(), fakeBranches{get: func(context.Context, uuid.UUID) (*branches.Branch, error) {
		return nil, branches.ErrNotFound
	}}, stubTx{})
	if _, err := svc.Create(context.Background(), uuid.New(), uuid.New(), uuid.New()); !errors.Is(err, ErrBranchNotFound) {
		t.Fatalf("Create = %v, want ErrBranchNotFound", err)
	}
}

func TestCreateMapsForeignKeyViolationDefensively(t *testing.T) {
	svc := NewService(fakeRepo{
		hasActiveByMember: func(context.Context, uuid.UUID) (bool, error) { return false, nil },
		create: func(context.Context, *Membership) error {
			return errors.New(`pq: insert violates foreign key "fk_memberships_branch" (SQLSTATE 23503)`)
		},
	}, emptyPackages(30), emptyMembers(), emptyBranches(), stubTx{})
	if _, err := svc.Create(context.Background(), uuid.New(), uuid.New(), uuid.New()); !errors.Is(err, ErrBranchNotFound) {
		t.Fatalf("Create = %v, want ErrBranchNotFound", err)
	}
}

func TestUpdateStateMachine(t *testing.T) {
	for _, tt := range []struct {
		name string
		from Status
		to   Status
		ok   bool
	}{
		{"active to frozen", StatusActive, StatusFrozen, true},
		{"active to expired", StatusActive, StatusExpired, true},
		{"active to active noop", StatusActive, StatusActive, true},
		{"frozen to active", StatusFrozen, StatusActive, true},
		{"frozen to expired", StatusFrozen, StatusExpired, true},
		{"frozen to frozen noop", StatusFrozen, StatusFrozen, true},
		{"expired to expired noop", StatusExpired, StatusExpired, true},
		{"expired to active refused", StatusExpired, StatusActive, false},
		{"expired to frozen refused", StatusExpired, StatusFrozen, false},
		{"active to active-cross refused", StatusActive, Status("suspended"), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var seen Status
			svc := NewService(fakeRepo{
				getByID: func(context.Context, uuid.UUID) (*Membership, error) {
					return &Membership{StartsAt: time.Now().UTC().Add(-time.Hour), ExpiresAt: time.Now().UTC().Add(30 * 24 * time.Hour), Status: tt.from}, nil
				},
				updateStatus: func(_ context.Context, _ uuid.UUID, expected Status, patch *Patch) error {
					seen = expected
					return nil
				},
			}, emptyPackages(30), emptyMembers(), emptyBranches(), stubTx{})
			to := tt.to
			_, err := svc.Update(context.Background(), uuid.New(), Patch{Status: &to})
			if tt.ok {
				if err != nil {
					t.Fatalf("Update %s -> %s: %v", tt.from, tt.to, err)
				}
				if seen != tt.from {
					t.Fatalf("UpdateStatus expected = %q, want observed %q", seen, tt.from)
				}
			} else if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Update %s -> %s = %v, want ErrInvalidInput", tt.from, tt.to, err)
			}
		})
	}
}

func TestUpdateLostUpdatePropagatesNotFound(t *testing.T) {
	svc := NewService(fakeRepo{
		getByID: func(context.Context, uuid.UUID) (*Membership, error) {
			return &Membership{StartsAt: time.Now().UTC().Add(-time.Hour), ExpiresAt: time.Now().UTC().Add(30 * 24 * time.Hour), Status: StatusActive}, nil
		},
		updateStatus: func(context.Context, uuid.UUID, Status, *Patch) error {
			return ErrNotFound
		},
	}, emptyPackages(30), emptyMembers(), emptyBranches(), stubTx{})
	frozen := StatusFrozen
	if _, err := svc.Update(context.Background(), uuid.New(), Patch{Status: &frozen}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update raced = %v, want ErrNotFound", err)
	}
}
