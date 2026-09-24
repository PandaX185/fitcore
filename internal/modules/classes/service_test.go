package classes

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/branches"
	"github.com/PandaX185/fitcore/internal/modules/trainers"
)

type fakeRepo struct {
	create  func(ctx context.Context, c *Class) error
	getByID func(ctx context.Context, id uuid.UUID) (*Class, error)
	update  func(ctx context.Context, id uuid.UUID, patch *Patch) error
	delete  func(ctx context.Context, id uuid.UUID) error
	list    func(ctx context.Context, branchID *uuid.UUID, trainerID *uuid.UUID) ([]*Class, error)
}

func (f fakeRepo) Create(ctx context.Context, c *Class) error {
	if f.create == nil {
		return nil
	}
	return f.create(ctx, c)
}
func (f fakeRepo) GetByID(ctx context.Context, id uuid.UUID) (*Class, error) {
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
func (f fakeRepo) List(ctx context.Context, branchID *uuid.UUID, trainerID *uuid.UUID) ([]*Class, error) {
	if f.list == nil {
		return nil, nil
	}
	return f.list(ctx, branchID, trainerID)
}

type fakeBranches struct {
	get func(ctx context.Context, id uuid.UUID) (*branches.Branch, error)
}

func (f fakeBranches) Get(ctx context.Context, id uuid.UUID) (*branches.Branch, error) {
	return f.get(ctx, id)
}

type fakeTrainers struct {
	get func(ctx context.Context, id uuid.UUID) (*trainers.Trainer, error)
}

func (f fakeTrainers) Get(ctx context.Context, id uuid.UUID) (*trainers.Trainer, error) {
	return f.get(ctx, id)
}

func existingBranch() fakeBranches {
	return fakeBranches{get: func(_ context.Context, id uuid.UUID) (*branches.Branch, error) {
		return &branches.Branch{ID: id}, nil
	}}
}

func TestCreate(t *testing.T) {
	branchID, trainerID := uuid.New(), uuid.New()
	var created *Class
	svc := NewService(fakeRepo{create: func(_ context.Context, c *Class) error {
		created = c
		return nil
	}}, existingBranch(), fakeTrainers{get: func(_ context.Context, id uuid.UUID) (*trainers.Trainer, error) {
		return &trainers.Trainer{ID: id, BranchID: branchID}, nil
	}})

	start := time.Now().UTC().Add(time.Hour)
	end := start.Add(time.Hour)
	got, err := svc.Create(context.Background(), branchID, &trainerID, "Spin", start, end, 20)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID != created.ID || created.Name != "Spin" || created.Capacity != 20 {
		t.Fatalf("class = %+v", created)
	}
	if created.TrainerID == nil || *created.TrainerID != trainerID {
		t.Fatalf("trainer = %v, want %v", created.TrainerID, trainerID)
	}
}

func TestCreateInvalid(t *testing.T) {
	start := time.Now().UTC().Add(time.Hour)
	id := uuid.New()
	svc := NewService(fakeRepo{}, existingBranch(), fakeTrainers{})
	tests := []struct {
		name     string
		branchID uuid.UUID
		nameArg  string
		start    time.Time
		end      time.Time
		capacity int
	}{
		{"nil branch", uuid.Nil, "Spin", start, start.Add(time.Hour), 20},
		{"blank name", id, " ", start, start.Add(time.Hour), 20},
		{"zero capacity", id, "Spin", start, start.Add(time.Hour), 0},
		{"ends before starts", id, "Spin", start, start.Add(-time.Hour), 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.Create(context.Background(), tt.branchID, nil, tt.nameArg, tt.start, tt.end, tt.capacity); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Create = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestCreateBranchNotFound(t *testing.T) {
	svc := NewService(fakeRepo{}, fakeBranches{get: func(context.Context, uuid.UUID) (*branches.Branch, error) {
		return nil, branches.ErrNotFound
	}}, fakeTrainers{})
	start := time.Now().UTC().Add(time.Hour)
	if _, err := svc.Create(context.Background(), uuid.New(), nil, "Spin", start, start.Add(time.Hour), 20); !errors.Is(err, ErrBranchNotFound) {
		t.Fatalf("Create = %v, want ErrBranchNotFound", err)
	}
}

func TestCreateTrainerNotFound(t *testing.T) {
	branchID := uuid.New()
	trainerID := uuid.New()
	svc := NewService(fakeRepo{}, existingBranch(), fakeTrainers{get: func(context.Context, uuid.UUID) (*trainers.Trainer, error) {
		return nil, trainers.ErrNotFound
	}})
	start := time.Now().UTC().Add(time.Hour)
	if _, err := svc.Create(context.Background(), branchID, &trainerID, "Spin", start, start.Add(time.Hour), 20); !errors.Is(err, ErrTrainerNotFound) {
		t.Fatalf("Create = %v, want ErrTrainerNotFound", err)
	}
}

func TestCreateTrainerWrongBranch(t *testing.T) {
	branchID := uuid.New()
	trainerID := uuid.New()
	svc := NewService(fakeRepo{}, existingBranch(), fakeTrainers{get: func(_ context.Context, id uuid.UUID) (*trainers.Trainer, error) {
		return &trainers.Trainer{ID: id, BranchID: uuid.New()}, nil
	}})
	start := time.Now().UTC().Add(time.Hour)
	if _, err := svc.Create(context.Background(), branchID, &trainerID, "Spin", start, start.Add(time.Hour), 20); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Create = %v, want ErrInvalidInput", err)
	}
}

func TestUpdate(t *testing.T) {
	id := uuid.New()
	start := time.Now().UTC().Add(time.Hour)
	svc := NewService(fakeRepo{
		getByID: func(_ context.Context, got uuid.UUID) (*Class, error) {
			return &Class{ID: got, StartsAt: start, EndsAt: start.Add(time.Hour), Capacity: 10}, nil
		},
		update: func(_ context.Context, got uuid.UUID, patch *Patch) error {
			if got != id || patch.Capacity == nil || *patch.Capacity != 30 {
				t.Fatalf("Update(%v, %+v)", got, patch)
			}
			return nil
		},
	}, existingBranch(), fakeTrainers{})
	capacity := 30
	got, err := svc.Update(context.Background(), id, Patch{Capacity: &capacity})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	_ = got
}

func TestUpdateInvalid(t *testing.T) {
	start := time.Now().UTC().Add(time.Hour)
	svc := NewService(fakeRepo{
		getByID: func(context.Context, uuid.UUID) (*Class, error) {
			return &Class{StartsAt: start, EndsAt: start.Add(time.Hour), Capacity: 10}, nil
		},
	}, existingBranch(), fakeTrainers{})
	ends := start.Add(-time.Hour)
	capacity := 0
	if _, err := svc.Update(context.Background(), uuid.Nil, Patch{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Update nil id = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.Update(context.Background(), uuid.New(), Patch{EndsAt: &ends}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Update shifted-end = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.Update(context.Background(), uuid.New(), Patch{Capacity: &capacity}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Update zero capacity = %v, want ErrInvalidInput", err)
	}
}

func TestDelete(t *testing.T) {
	id := uuid.New()
	svc := NewService(fakeRepo{delete: func(_ context.Context, got uuid.UUID) error {
		if got != id {
			t.Fatalf("Delete = %v, want %v", got, id)
		}
		return nil
	}}, existingBranch(), fakeTrainers{})
	if err := svc.Delete(context.Background(), id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := svc.Delete(context.Background(), uuid.Nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Delete nil = %v, want ErrInvalidInput", err)
	}
}

func TestList(t *testing.T) {
	branchID := uuid.New()
	trainerID := uuid.New()
	svc := NewService(fakeRepo{list: func(_ context.Context, b, tr *uuid.UUID) ([]*Class, error) {
		if b == nil || *b != branchID || tr == nil || *tr != trainerID {
			t.Fatalf("List(%v, %v)", b, tr)
		}
		return []*Class{{ID: uuid.New()}}, nil
	}}, existingBranch(), fakeTrainers{})
	got, err := svc.List(context.Background(), &branchID, &trainerID)
	if err != nil || len(got) != 1 {
		t.Fatalf("List = %v, %v", got, err)
	}
}

func TestListNilBranchAllowed(t *testing.T) {
	branchID := uuid.New()
	svc := NewService(fakeRepo{list: func(_ context.Context, b, tr *uuid.UUID) ([]*Class, error) {
		if b != nil || tr != nil {
			t.Fatalf("List(%v, %v), want all", b, tr)
		}
		return []*Class{{ID: branchID}}, nil
	}}, existingBranch(), fakeTrainers{})
	if _, err := svc.List(context.Background(), nil, nil); err != nil {
		t.Fatalf("List nil filters: %v", err)
	}
}
