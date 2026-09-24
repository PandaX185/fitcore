package packages

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeRepo struct {
	create  func(ctx context.Context, p *Package) error
	getByID func(ctx context.Context, id uuid.UUID) (*Package, error)
	list    func(ctx context.Context) ([]*Package, error)
	update  func(ctx context.Context, id uuid.UUID, patch *Patch) error
}

func (f fakeRepo) Create(ctx context.Context, p *Package) error {
	if f.create == nil {
		return nil
	}
	return f.create(ctx, p)
}
func (f fakeRepo) GetByID(ctx context.Context, id uuid.UUID) (*Package, error) {
	return f.getByID(ctx, id)
}
func (f fakeRepo) List(ctx context.Context) ([]*Package, error) {
	if f.list == nil {
		return nil, nil
	}
	return f.list(ctx)
}
func (f fakeRepo) Update(ctx context.Context, id uuid.UUID, patch *Patch) error {
	if f.update == nil {
		return nil
	}
	return f.update(ctx, id, patch)
}

func TestGet(t *testing.T) {
	id := uuid.New()
	sentinel := &Package{ID: id}
	svc := NewService(fakeRepo{getByID: func(_ context.Context, got uuid.UUID) (*Package, error) {
		if got != id {
			t.Fatalf("GetByID id = %v, want %v", got, id)
		}
		return sentinel, nil
	}})
	got, err := svc.Get(context.Background(), id)
	if err != nil || got != sentinel {
		t.Fatalf("Get = %v, %v; want %v", got, err, sentinel)
	}
}

func TestGetNilID(t *testing.T) {
	svc := NewService(fakeRepo{})
	if _, err := svc.Get(context.Background(), uuid.Nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Get nil id = %v, want ErrInvalid", err)
	}
}

func TestCreate(t *testing.T) {
	var created *Package
	svc := NewService(fakeRepo{create: func(_ context.Context, p *Package) error {
		created = p
		return nil
	}})
	got, err := svc.Create(context.Background(), "Monthly", 30, 25000, "bhd")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID != created.ID || !created.Active {
		t.Fatalf("Create = %+v", created)
	}
	if created.Currency != "BHD" {
		t.Fatalf("currency = %q, want BHD (normalized)", created.Currency)
	}
	if created.DurationDays != 30 || created.PriceCents != 25000 {
		t.Fatalf("package = %+v", created)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("timestamps not set: %+v", created)
	}
}

func TestCreateInvalid(t *testing.T) {
	tests := []struct {
		name         string
		nameArg      string
		durationDays int
		priceCents   int64
		currency     string
	}{
		{"blank name", " ", 30, 25000, "BHD"},
		{"neg duration", "Monthly", 0, 25000, "BHD"},
		{"neg price", "Monthly", 30, -1, "BHD"},
		{"short currency", "Monthly", 30, 25000, "BH"},
		{"long currency", "Monthly", 30, 25000, "BHDX"},
		{"lowercase digits", "Monthly", 30, 25000, "1HD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(fakeRepo{})
			if _, err := svc.Create(context.Background(), tt.nameArg, tt.durationDays, tt.priceCents, tt.currency); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Create = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestCreateDuplicate(t *testing.T) {
	svc := NewService(fakeRepo{create: func(context.Context, *Package) error {
		return ErrDuplicateName
	}})
	if _, err := svc.Create(context.Background(), "Monthly", 30, 25000, "BHD"); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("Create dup = %v, want ErrDuplicateName", err)
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
		getByID: func(_ context.Context, id uuid.UUID) (*Package, error) {
			return &Package{ID: id, Active: false}, nil
		},
	})
	active := false
	got, err := svc.Update(context.Background(), id, Patch{Active: &active})
	if err != nil || got.Active {
		t.Fatalf("Update = %v, %v", got, err)
	}
}

func TestUpdateInvalid(t *testing.T) {
	tests := []struct {
		name  string
		id    uuid.UUID
		patch Patch
	}{
		{"nil id", uuid.Nil, Patch{}},
		{"blank name", uuid.New(), Patch{Name: ptr(" ")}},
		{"zero duration", uuid.New(), Patch{DurationDays: ptr(0)}},
		{"neg price", uuid.New(), Patch{PriceCents: ptr(int64(-1))}},
		{"bad currency", uuid.New(), Patch{Currency: ptr("XX")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(fakeRepo{})
			if _, err := svc.Update(context.Background(), tt.id, tt.patch); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Update = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestList(t *testing.T) {
	want := []*Package{{ID: uuid.New()}}
	svc := NewService(fakeRepo{list: func(context.Context) ([]*Package, error) {
		return want, nil
	}})
	got, err := svc.List(context.Background())
	if err != nil || len(got) != 1 {
		t.Fatalf("List = %v, %v", got, err)
	}
}

func TestExpiryUsesDuration(t *testing.T) {
	// Guards the AddDate(0, 0, days) calendar-day semantics used by memberships.
	start := time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC)
	got := start.AddDate(0, 0, 30)
	if want := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("AddDate(30d) = %v, want %v", got, want)
	}
}

func ptr[T any](v T) *T { return &v }
