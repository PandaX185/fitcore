package trainers

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/branches"
)

type fakeRepo struct {
	create       func(ctx context.Context, t *Trainer) error
	getByID      func(ctx context.Context, id uuid.UUID) (*Trainer, error)
	listByBranch func(ctx context.Context, branchID uuid.UUID) ([]*Trainer, error)
	update       func(ctx context.Context, id uuid.UUID, patch *Patch) error
}

func (f fakeRepo) Create(ctx context.Context, t *Trainer) error {
	if f.create == nil {
		return nil
	}
	return f.create(ctx, t)
}
func (f fakeRepo) GetByID(ctx context.Context, id uuid.UUID) (*Trainer, error) {
	return f.getByID(ctx, id)
}
func (f fakeRepo) ListByBranch(ctx context.Context, branchID uuid.UUID) ([]*Trainer, error) {
	if f.listByBranch == nil {
		return nil, nil
	}
	return f.listByBranch(ctx, branchID)
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
	var created *Trainer
	svc := NewService(fakeRepo{create: func(_ context.Context, tr *Trainer) error {
		created = tr
		return nil
	}}, existingBranch())

	got, err := svc.Create(context.Background(), branchID, "Sana", "  Sana@Example.COM ", "555")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID != created.ID || !created.Active || created.BranchID != branchID {
		t.Fatalf("trainer = %+v", created)
	}
	if created.Email != "sana@example.com" {
		t.Fatalf("email = %q, want normalized", created.Email)
	}
}

func TestCreateInvalid(t *testing.T) {
	svc := NewService(fakeRepo{}, existingBranch())
	tests := []struct {
		name     string
		branchID uuid.UUID
		nameArg  string
		email    string
	}{
		{"nil branch", uuid.Nil, "Sana", "s@b.com"},
		{"blank name", uuid.New(), " ", "s@b.com"},
		{"blank email", uuid.New(), "Sana", ""},
		{"no at sign", uuid.New(), "Sana", "nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.Create(context.Background(), tt.branchID, tt.nameArg, tt.email, "555"); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Create = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestCreateBranchNotFound(t *testing.T) {
	svc := NewService(fakeRepo{}, fakeBranches{get: func(context.Context, uuid.UUID) (*branches.Branch, error) {
		return nil, branches.ErrNotFound
	}})
	if _, err := svc.Create(context.Background(), uuid.New(), "Sana", "s@b.com", ""); !errors.Is(err, ErrBranchNotFound) {
		t.Fatalf("Create = %v, want ErrBranchNotFound", err)
	}
}

func TestCreateDuplicateEmail(t *testing.T) {
	svc := NewService(fakeRepo{create: func(context.Context, *Trainer) error {
		return ErrDuplicateEmail
	}}, existingBranch())
	if _, err := svc.Create(context.Background(), uuid.New(), "Sana", "s@b.com", ""); !errors.Is(err, ErrDuplicateEmail) {
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
		getByID: func(_ context.Context, id uuid.UUID) (*Trainer, error) {
			return &Trainer{ID: id, Active: false}, nil
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
	want := []*Trainer{{ID: uuid.New()}}
	svc := NewService(fakeRepo{listByBranch: func(_ context.Context, got uuid.UUID) ([]*Trainer, error) {
		if got != branchID {
			t.Fatalf("ListByBranch = %v, want %v", got, branchID)
		}
		return want, nil
	}}, existingBranch())
	got, err := svc.ListByBranch(context.Background(), branchID)
	if err != nil || len(got) != 1 {
		t.Fatalf("ListByBranch = %v, %v", got, err)
	}
}

func TestListByBranchNilID(t *testing.T) {
	svc := NewService(fakeRepo{}, existingBranch())
	if _, err := svc.ListByBranch(context.Background(), uuid.Nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ListByBranch nil = %v, want ErrInvalidInput", err)
	}
}

func ptr[T any](v T) *T { return &v }
