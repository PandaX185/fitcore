package staff

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/auth"
	"github.com/PandaX185/fitcore/internal/modules/branches"
	"github.com/PandaX185/fitcore/internal/paging"
)

type fakeRepo struct {
	create       func(ctx context.Context, s *Staff) error
	getByID      func(ctx context.Context, id uuid.UUID) (*Staff, error)
	listByBranch func(ctx context.Context, q *BranchListQuery) ([]*Staff, error)
	update       func(ctx context.Context, id uuid.UUID, patch *Patch) error
}

func (f fakeRepo) Create(ctx context.Context, s *Staff) error {
	if f.create == nil {
		return nil
	}
	return f.create(ctx, s)
}
func (f fakeRepo) GetByID(ctx context.Context, id uuid.UUID) (*Staff, error) {
	return f.getByID(ctx, id)
}
func (f fakeRepo) ListByBranch(ctx context.Context, q *BranchListQuery) ([]*Staff, error) {
	if f.listByBranch == nil {
		return nil, nil
	}
	return f.listByBranch(ctx, q)
}
func (f fakeRepo) Update(ctx context.Context, id uuid.UUID, patch *Patch) error {
	if f.update == nil {
		return nil
	}
	return f.update(ctx, id, patch)
}

type fakeBranches struct {
	get func(ctx context.Context, id uuid.UUID) (*branches.Branch, error)
}

func (f fakeBranches) Get(ctx context.Context, id uuid.UUID) (*branches.Branch, error) {
	return f.get(ctx, id)
}

func existingBranch() fakeBranches {
	return fakeBranches{get: func(_ context.Context, id uuid.UUID) (*branches.Branch, error) {
		return &branches.Branch{ID: id}, nil
	}}
}

func TestCreate(t *testing.T) {
	branchID := uuid.New()
	var created *Staff
	svc := NewService(fakeRepo{create: func(_ context.Context, s *Staff) error {
		created = s
		return nil
	}}, existingBranch())

	got, err := svc.Create(context.Background(), branchID, "Ada", "  Ada@Example.COM ", "555", []auth.Permission{auth.PermBranchesRead})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID != created.ID || !created.Active || created.BranchID != branchID {
		t.Fatalf("staff = %+v", created)
	}
	if created.Email != "ada@example.com" {
		t.Fatalf("email = %q, want normalized", created.Email)
	}
}

func TestCreateInvalid(t *testing.T) {
	svc := NewService(fakeRepo{}, existingBranch())
	tests := []struct {
		name        string
		branchID    uuid.UUID
		nameArg     string
		email       string
		permissions []auth.Permission
	}{
		{"nil branch", uuid.Nil, "Ada", "a@b.com", nil},
		{"blank name", uuid.New(), " ", "a@b.com", nil},
		{"blank email", uuid.New(), "Ada", "", nil},
		{"no at sign", uuid.New(), "Ada", "nope", nil},
		{"unknown permission", uuid.New(), "Ada", "a@b.com", []auth.Permission{"hack:all"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), tt.branchID, tt.nameArg, tt.email, "555", tt.permissions)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Create = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestCreateBranchNotFound(t *testing.T) {
	svc := NewService(fakeRepo{}, fakeBranches{get: func(context.Context, uuid.UUID) (*branches.Branch, error) {
		return nil, branches.ErrNotFound
	}})
	if _, err := svc.Create(context.Background(), uuid.New(), "Ada", "a@b.com", "", nil); !errors.Is(err, ErrBranchNotFound) {
		t.Fatalf("Create = %v, want ErrBranchNotFound", err)
	}
}

func TestCreateDuplicateEmail(t *testing.T) {
	svc := NewService(fakeRepo{create: func(context.Context, *Staff) error {
		return ErrDuplicateEmail
	}}, existingBranch())
	if _, err := svc.Create(context.Background(), uuid.New(), "Ada", "a@b.com", "", nil); !errors.Is(err, ErrDuplicateEmail) {
		t.Fatalf("Create = %v, want ErrDuplicateEmail", err)
	}
}

func TestUpdate(t *testing.T) {
	id := uuid.New()
	svc := NewService(fakeRepo{
		update: func(_ context.Context, got uuid.UUID, patch *Patch) error {
			if got != id || patch.Active == nil || *patch.Active {
				t.Fatalf("Update(%v, %+v)", got, patch)
			}
			return nil
		},
		getByID: func(_ context.Context, id uuid.UUID) (*Staff, error) {
			return &Staff{ID: id, Active: false}, nil
		},
	}, existingBranch())
	active := false
	got, err := svc.Update(context.Background(), id, Patch{Active: &active})
	if err != nil || got.Active {
		t.Fatalf("Update = %v, %v", got, err)
	}
}

func TestUpdateInvalid(t *testing.T) {
	svc := NewService(fakeRepo{}, existingBranch())
	tests := []struct {
		name  string
		patch Patch
	}{
		{"nil id", Patch{}},
		{"blank name", Patch{Name: ptr(" ")}},
		{"blank email", Patch{Email: ptr("")}},
		{"no at sign", Patch{Email: ptr("nope")}},
		{"unknown permission", Patch{Permissions: &[]auth.Permission{"hack:all"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.Update(context.Background(), uuid.Nil, tt.patch); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Update = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestListByBranch(t *testing.T) {
	branchID := uuid.New()
	want := []*Staff{{ID: uuid.New()}}
	svc := NewService(fakeRepo{listByBranch: func(_ context.Context, q *BranchListQuery) ([]*Staff, error) {
		if q.BranchID != branchID {
			t.Fatalf("BranchListQuery(%+v)", q)
		}
		return want, nil
	}}, existingBranch())
	res, err := svc.ListByBranch(context.Background(), branchID, BranchListParams{})
	if err != nil || len(res.Items) != 1 {
		t.Fatalf("ListByBranch = %v, %v", res, err)
	}
}

func TestListByBranchNilID(t *testing.T) {
	svc := NewService(fakeRepo{}, existingBranch())
	if _, err := svc.ListByBranch(context.Background(), uuid.Nil, BranchListParams{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ListByBranch nil = %v, want ErrInvalidInput", err)
	}
}

func TestListByBranchFirstPage(t *testing.T) {
	branchID := uuid.New()
	svc := NewService(fakeRepo{
		listByBranch: func(_ context.Context, q *BranchListQuery) ([]*Staff, error) {
			if q.Limit != paging.DefaultLimit+1 {
				t.Fatalf("Limit = %d, want %d", q.Limit, paging.DefaultLimit+1)
			}
			if q.AfterName != "" || q.AfterID != uuid.Nil {
				t.Fatalf("cursor start = %q/%v, want empty", q.AfterName, q.AfterID)
			}
			items := []*Staff{}
			for i := 0; i < paging.DefaultLimit+1; i++ {
				items = append(items, &Staff{ID: uuid.New(), Name: "B"})
			}
			return items, nil
		},
	}, existingBranch())

	res, err := svc.ListByBranch(context.Background(), branchID, BranchListParams{})
	if err != nil {
		t.Fatalf("ListByBranch: unexpected error %v", err)
	}
	if len(res.Items) != paging.DefaultLimit {
		t.Fatalf("items = %d, want %d", len(res.Items), paging.DefaultLimit)
	}
	if res.NextCursor == "" {
		t.Fatal("NextCursor empty, want a value when a page is full")
	}
}

func TestListByBranchLastPage(t *testing.T) {
	svc := NewService(fakeRepo{
		listByBranch: func(context.Context, *BranchListQuery) ([]*Staff, error) {
			return []*Staff{{ID: uuid.New(), Name: "Only"}}, nil
		},
	}, existingBranch())

	res, err := svc.ListByBranch(context.Background(), uuid.New(), BranchListParams{Limit: 20})
	if err != nil {
		t.Fatalf("ListByBranch: unexpected error %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(res.Items))
	}
	if res.NextCursor != "" {
		t.Fatalf("NextCursor = %q, want empty on the last page", res.NextCursor)
	}
}

func TestListByBranchClampsLimit(t *testing.T) {
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
				listByBranch: func(_ context.Context, q *BranchListQuery) ([]*Staff, error) {
					if q.Limit != tt.want+1 {
						t.Fatalf("repo Limit = %d, want %d", q.Limit, tt.want+1)
					}
					return nil, nil
				},
			}, existingBranch())
			if _, err := svc.ListByBranch(context.Background(), uuid.New(), BranchListParams{Limit: tt.in}); err != nil {
				t.Fatalf("ListByBranch: unexpected error %v", err)
			}
		})
	}
}

func TestListByBranchCursorRoundTrip(t *testing.T) {
	const name = "Next"
	id := uuid.New()
	cursor := paging.Cursor{Key: name, ID: id}.Encode()

	svc := NewService(fakeRepo{
		listByBranch: func(_ context.Context, q *BranchListQuery) ([]*Staff, error) {
			if q.AfterName != name {
				t.Fatalf("AfterName = %q, want %q", q.AfterName, name)
			}
			if q.AfterID != id {
				t.Fatalf("AfterID = %v, want %v", q.AfterID, id)
			}
			return nil, nil
		},
	}, existingBranch())

	if _, err := svc.ListByBranch(context.Background(), uuid.New(), BranchListParams{Cursor: cursor}); err != nil {
		t.Fatalf("ListByBranch: unexpected error %v", err)
	}
}

func TestListByBranchRejectsBadCursor(t *testing.T) {
	for _, cur := range []string{"%%%", "not-base64-!", "eyJuYW1lIjoibiJ9"} {
		t.Run(cur, func(t *testing.T) {
			svc := NewService(fakeRepo{
				listByBranch: func(context.Context, *BranchListQuery) ([]*Staff, error) {
					t.Fatal("repo must not be called for a bad cursor")
					return nil, nil
				},
			}, existingBranch())
			_, err := svc.ListByBranch(context.Background(), uuid.New(), BranchListParams{Cursor: cur})
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("ListByBranch error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }
