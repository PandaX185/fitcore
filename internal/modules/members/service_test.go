package members

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type fakeRepo struct {
	create  func(ctx context.Context, m *Member) error
	getByID func(ctx context.Context, id uuid.UUID) (*Member, error)
	update  func(ctx context.Context, id uuid.UUID, patch *Patch) error
	delete  func(ctx context.Context, id uuid.UUID) error
	list    func(ctx context.Context) ([]*Member, error)
}

func (f fakeRepo) Create(ctx context.Context, m *Member) error {
	if f.create == nil {
		return nil
	}
	return f.create(ctx, m)
}
func (f fakeRepo) GetByID(ctx context.Context, id uuid.UUID) (*Member, error) {
	return f.getByID(ctx, id)
}
func (f fakeRepo) Update(ctx context.Context, id uuid.UUID, patch *Patch) error {
	if f.update == nil {
		return nil
	}
	return f.update(ctx, id, patch)
}
func (f fakeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if f.delete == nil {
		return nil
	}
	return f.delete(ctx, id)
}
func (f fakeRepo) List(ctx context.Context) ([]*Member, error) {
	if f.list == nil {
		return nil, nil
	}
	return f.list(ctx)
}

func TestServiceGet(t *testing.T) {
	id := uuid.New()
	sentinel := &Member{ID: id}
	svc := NewService(fakeRepo{
		getByID: func(ctx context.Context, got uuid.UUID) (*Member, error) {
			if got != id {
				t.Fatalf("GetByID id = %v, want %v", got, id)
			}
			return sentinel, nil
		},
	})

	got, err := svc.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if got != sentinel {
		t.Fatalf("Get = %v, want %v", got, sentinel)
	}
}

func TestServiceGetNilID(t *testing.T) {
	svc := NewService(fakeRepo{})
	if _, err := svc.Get(context.Background(), uuid.Nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Get nil id = %v, want ErrInvalidInput", err)
	}
}

func TestServiceGetNotFound(t *testing.T) {
	svc := NewService(fakeRepo{
		getByID: func(context.Context, uuid.UUID) (*Member, error) {
			return nil, ErrNotFound
		},
	})
	if _, err := svc.Get(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get missing = %v, want ErrNotFound", err)
	}
}

func TestServiceCreate(t *testing.T) {
	branchID := uuid.New()
	var created *Member
	svc := NewService(fakeRepo{
		create: func(ctx context.Context, m *Member) error {
			created = m
			return nil
		},
	})

	got, err := svc.Create(context.Background(), branchID, "Ada", "ada@example.com", "123")
	if err != nil {
		t.Fatalf("Create: unexpected error %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("Create returned %v, repo saw %v", got.ID, created.ID)
	}
	if created.BranchID != branchID {
		t.Fatalf("branchID = %v, want %v", created.BranchID, branchID)
	}
	if created.Status != StatusActive {
		t.Fatalf("status = %q, want %q", created.Status, StatusActive)
	}
	if created.Email != "ada@example.com" {
		t.Fatalf("email = %q", created.Email)
	}
}

func TestServiceCreateInvalid(t *testing.T) {
	branchID := uuid.New()
	tests := []struct {
		name     string
		branchID uuid.UUID
		member   string
		email    string
	}{
		{"nil branch", uuid.Nil, "Ada", "ada@example.com"},
		{"blank name", branchID, " ", "ada@example.com"},
		{"blank email", branchID, "Ada", ""},
		{"no at sign", branchID, "Ada", "not-an-email"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(fakeRepo{})
			_, err := svc.Create(context.Background(), tt.branchID, tt.member, tt.email, "")
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Create = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestServiceCreateNormalizesEmail(t *testing.T) {
	branchID := uuid.New()
	var created *Member
	svc := NewService(fakeRepo{
		create: func(ctx context.Context, m *Member) error {
			created = m
			return nil
		},
	})
	if _, err := svc.Create(context.Background(), branchID, "Ada", "  Ada@Example.COM  ", ""); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Email != "ada@example.com" {
		t.Fatalf("email = %q, want %q", created.Email, "ada@example.com")
	}
}

func TestServiceCreateDuplicate(t *testing.T) {
	svc := NewService(fakeRepo{
		create: func(context.Context, *Member) error {
			return ErrDuplicateEmail
		},
	})
	_, err := svc.Create(context.Background(), uuid.New(), "Ada", "ada@example.com", "")
	if !errors.Is(err, ErrDuplicateEmail) {
		t.Fatalf("Create dup = %v, want ErrDuplicateEmail", err)
	}
}

func TestServiceUpdate(t *testing.T) {
	id := uuid.New()
	svc := NewService(fakeRepo{
		update: func(ctx context.Context, got uuid.UUID, patch *Patch) error {
			if got != id {
				t.Fatalf("Update id = %v, want %v", got, id)
			}
			if patch.Status == nil || *patch.Status != StatusSuspended {
				t.Fatalf("patch = %+v", patch)
			}
			return nil
		},
		getByID: func(ctx context.Context, id uuid.UUID) (*Member, error) {
			return &Member{ID: id, Status: StatusSuspended}, nil
		},
	})
	s := StatusSuspended
	got, err := svc.Update(context.Background(), id, Patch{Status: &s})
	if err != nil {
		t.Fatalf("Update: unexpected error %v", err)
	}
	if got.Status != StatusSuspended {
		t.Fatalf("status = %q, want %q", got.Status, StatusSuspended)
	}
}

func TestServiceUpdateInvalid(t *testing.T) {
	tests := []struct {
		name  string
		id    uuid.UUID
		patch Patch
	}{
		{"nil id", uuid.Nil, Patch{}},
		{"blank name", uuid.New(), Patch{Name: strPtr(" ")}},
		{"blank email", uuid.New(), Patch{Email: strPtr("")}},
		{"no at sign", uuid.New(), Patch{Email: strPtr("nope")}},
		{"bad status", uuid.New(), Patch{Status: statusPtr("frozen")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(fakeRepo{})
			_, err := svc.Update(context.Background(), tt.id, tt.patch)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Update = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestServiceUpdateDuplicate(t *testing.T) {
	svc := NewService(fakeRepo{
		update: func(context.Context, uuid.UUID, *Patch) error {
			return ErrDuplicateEmail
		},
	})
	_, err := svc.Update(context.Background(), uuid.New(), Patch{})
	if !errors.Is(err, ErrDuplicateEmail) {
		t.Fatalf("Update dup = %v, want ErrDuplicateEmail", err)
	}
}

func TestServiceDelete(t *testing.T) {
	id := uuid.New()
	svc := NewService(fakeRepo{
		delete: func(ctx context.Context, got uuid.UUID) error {
			if got != id {
				t.Fatalf("Delete id = %v, want %v", got, id)
			}
			return nil
		},
	})
	if err := svc.Delete(context.Background(), id); err != nil {
		t.Fatalf("Delete: unexpected error %v", err)
	}
}

func TestServiceDeleteNilID(t *testing.T) {
	svc := NewService(fakeRepo{})
	if err := svc.Delete(context.Background(), uuid.Nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Delete nil id = %v, want ErrInvalidInput", err)
	}
}

func TestServiceDeleteNotFound(t *testing.T) {
	svc := NewService(fakeRepo{
		delete: func(context.Context, uuid.UUID) error {
			return ErrNotFound
		},
	})
	if err := svc.Delete(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete missing = %v, want ErrNotFound", err)
	}
}

func TestServiceList(t *testing.T) {
	want := []*Member{{ID: uuid.New()}, {ID: uuid.New()}}
	svc := NewService(fakeRepo{
		list: func(context.Context) ([]*Member, error) {
			return want, nil
		},
	})
	got, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: unexpected error %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("List = %d items, want %d", len(got), len(want))
	}
}

func strPtr(s string) *string { return &s }
func statusPtr(s string) *Status {
	v := Status(s)
	return &v
}
