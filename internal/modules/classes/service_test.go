package classes

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/branches"
	"github.com/PandaX185/fitcore/internal/modules/trainers"
	"github.com/PandaX185/fitcore/internal/paging"
)

type fakeRepo struct {
	create  func(ctx context.Context, c *Class) error
	getByID func(ctx context.Context, id uuid.UUID) (*Class, error)
	update  func(ctx context.Context, id uuid.UUID, patch *Patch) error
	delete  func(ctx context.Context, id uuid.UUID) error
	list    func(ctx context.Context, q *ListQuery) ([]*Class, error)
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

// GetForUpdate mirrors GetByID: fakes hold no locks.
func (f fakeRepo) GetForUpdate(ctx context.Context, id uuid.UUID) (*Class, error) {
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
func (f fakeRepo) List(ctx context.Context, q *ListQuery) ([]*Class, error) {
	if f.list == nil {
		return nil, nil
	}
	return f.list(ctx, q)
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

type fakeOccupancy struct {
	count func(ctx context.Context, classID uuid.UUID) (int, error)
}

func (f fakeOccupancy) CountActiveByClass(ctx context.Context, classID uuid.UUID) (int, error) {
	if f.count == nil {
		return 0, nil
	}
	return f.count(ctx, classID)
}

type stubTx struct{}

func (stubTx) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func TestCreate(t *testing.T) {
	branchID, trainerID := uuid.New(), uuid.New()
	var created *Class
	svc := NewService(fakeRepo{create: func(_ context.Context, c *Class) error {
		created = c
		return nil
	}}, existingBranch(), fakeTrainers{get: func(_ context.Context, id uuid.UUID) (*trainers.Trainer, error) {
		return &trainers.Trainer{ID: id, BranchID: branchID}, nil
	}}, fakeOccupancy{}, stubTx{})

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
	svc := NewService(fakeRepo{}, existingBranch(), fakeTrainers{}, fakeOccupancy{}, stubTx{})
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
	}}, fakeTrainers{}, fakeOccupancy{}, stubTx{})
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
	}}, fakeOccupancy{}, stubTx{})
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
	}}, fakeOccupancy{}, stubTx{})
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
	}, existingBranch(), fakeTrainers{}, fakeOccupancy{}, stubTx{})
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
	}, existingBranch(), fakeTrainers{}, fakeOccupancy{}, stubTx{})
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
	}}, existingBranch(), fakeTrainers{}, fakeOccupancy{}, stubTx{})
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
	svc := NewService(fakeRepo{list: func(_ context.Context, q *ListQuery) ([]*Class, error) {
		if q.BranchID == nil || *q.BranchID != branchID || q.TrainerID == nil || *q.TrainerID != trainerID {
			t.Fatalf("List(%+v)", q)
		}
		return []*Class{{ID: uuid.New()}}, nil
	}}, existingBranch(), fakeTrainers{}, fakeOccupancy{}, stubTx{})
	res, err := svc.List(context.Background(), ListParams{BranchID: &branchID, TrainerID: &trainerID})
	if err != nil || len(res.Items) != 1 {
		t.Fatalf("List = %v, %v", res, err)
	}
}

func TestListNilBranchAllowed(t *testing.T) {
	branchID := uuid.New()
	svc := NewService(fakeRepo{list: func(_ context.Context, q *ListQuery) ([]*Class, error) {
		if q.BranchID != nil || q.TrainerID != nil {
			t.Fatalf("List(%+v), want all", q)
		}
		return []*Class{{ID: branchID}}, nil
	}}, existingBranch(), fakeTrainers{}, fakeOccupancy{}, stubTx{})
	if _, err := svc.List(context.Background(), ListParams{}); err != nil {
		t.Fatalf("List nil filters: %v", err)
	}
}

func TestListFirstPage(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	svc := NewService(fakeRepo{
		list: func(_ context.Context, q *ListQuery) ([]*Class, error) {
			if q.Limit != paging.DefaultLimit+1 {
				t.Fatalf("Limit = %d, want %d", q.Limit, paging.DefaultLimit+1)
			}
			if q.AfterStartsAt != "" || q.AfterID != uuid.Nil {
				t.Fatalf("cursor start = %q/%v, want empty", q.AfterStartsAt, q.AfterID)
			}
			items := []*Class{}
			for i := 0; i < paging.DefaultLimit+1; i++ {
				items = append(items, &Class{ID: uuid.New(), StartsAt: now})
			}
			return items, nil
		},
	}, existingBranch(), fakeTrainers{}, fakeOccupancy{}, stubTx{})

	res, err := svc.List(context.Background(), ListParams{})
	if err != nil {
		t.Fatalf("List: unexpected error %v", err)
	}
	if len(res.Items) != paging.DefaultLimit {
		t.Fatalf("items = %d, want %d", len(res.Items), paging.DefaultLimit)
	}
	if res.NextCursor == "" {
		t.Fatal("NextCursor empty, want a value when a page is full")
	}
	// The emitted cursor must round-trip to the same key the repo consumes.
	cur, err := paging.DecodeCursor(res.NextCursor)
	if err != nil {
		t.Fatalf("decode NextCursor: %v", err)
	}
	if cur.Key != now.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("cursor key = %q, want %q", cur.Key, now.UTC().Format(time.RFC3339Nano))
	}
}

func TestListLastPage(t *testing.T) {
	svc := NewService(fakeRepo{
		list: func(context.Context, *ListQuery) ([]*Class, error) {
			return []*Class{{ID: uuid.New()}}, nil
		},
	}, existingBranch(), fakeTrainers{}, fakeOccupancy{}, stubTx{})

	res, err := svc.List(context.Background(), ListParams{Limit: 20})
	if err != nil {
		t.Fatalf("List: unexpected error %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(res.Items))
	}
	if res.NextCursor != "" {
		t.Fatalf("NextCursor = %q, want empty on the last page", res.NextCursor)
	}
}

func TestListClampsLimit(t *testing.T) {
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
				list: func(_ context.Context, q *ListQuery) ([]*Class, error) {
					if q.Limit != tt.want+1 {
						t.Fatalf("repo Limit = %d, want %d", q.Limit, tt.want+1)
					}
					return nil, nil
				},
			}, existingBranch(), fakeTrainers{}, fakeOccupancy{}, stubTx{})
			if _, err := svc.List(context.Background(), ListParams{Limit: tt.in}); err != nil {
				t.Fatalf("List: unexpected error %v", err)
			}
		})
	}
}

func TestListCursorRoundTrip(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Microsecond)
	id := uuid.New()
	cursor := paging.Cursor{Key: at.Format(time.RFC3339Nano), ID: id}.Encode()

	svc := NewService(fakeRepo{
		list: func(_ context.Context, q *ListQuery) ([]*Class, error) {
			if q.AfterStartsAt != at.Format(time.RFC3339Nano) {
				t.Fatalf("AfterStartsAt = %q, want %q", q.AfterStartsAt, at.Format(time.RFC3339Nano))
			}
			if q.AfterID != id {
				t.Fatalf("AfterID = %v, want %v", q.AfterID, id)
			}
			return nil, nil
		},
	}, existingBranch(), fakeTrainers{}, fakeOccupancy{}, stubTx{})

	if _, err := svc.List(context.Background(), ListParams{Cursor: cursor}); err != nil {
		t.Fatalf("List: unexpected error %v", err)
	}
}

func TestListRejectsBadCursor(t *testing.T) {
	for _, cur := range []string{"%%%", "not-base64-!", "eyJuYW1lIjoibiJ9"} {
		t.Run(cur, func(t *testing.T) {
			svc := NewService(fakeRepo{
				list: func(context.Context, *ListQuery) ([]*Class, error) {
					t.Fatal("repo must not be called for a bad cursor")
					return nil, nil
				},
			}, existingBranch(), fakeTrainers{}, fakeOccupancy{}, stubTx{})
			_, err := svc.List(context.Background(), ListParams{Cursor: cur})
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("List error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestUpdateShrinkGuard(t *testing.T) {
	start := time.Now().UTC().Add(time.Hour)
	newSvc := func(t *testing.T, occupancy int, updated *bool) *Service {
		return NewService(fakeRepo{
			getByID: func(context.Context, uuid.UUID) (*Class, error) {
				return &Class{StartsAt: start, EndsAt: start.Add(time.Hour), Capacity: 10}, nil
			},
			update: func(context.Context, uuid.UUID, *Patch) error {
				*updated = true
				return nil
			},
		}, existingBranch(), fakeTrainers{}, fakeOccupancy{count: func(context.Context, uuid.UUID) (int, error) {
			return occupancy, nil
		}}, stubTx{})
	}
	shrink := 3
	updated := false
	if _, err := newSvc(t, 5, &updated).Update(context.Background(), uuid.New(), Patch{Capacity: &shrink}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Update shrink below occupancy = %v, want ErrInvalidInput", err)
	}
	if updated {
		t.Fatal("repo.Update called despite rejected shrink")
	}
	// Shrinking exactly to the live count is allowed.
	exact := 5
	updated = false
	if _, err := newSvc(t, 5, &updated).Update(context.Background(), uuid.New(), Patch{Capacity: &exact}); err != nil {
		t.Fatalf("Update shrink to occupancy: %v", err)
	}
	if !updated {
		t.Fatal("repo.Update not called for an allowed shrink")
	}
}

func TestDeleteWithBookingsConflicts(t *testing.T) {
	var deleted bool
	svc := NewService(fakeRepo{
		delete: func(context.Context, uuid.UUID) error {
			deleted = true
			return nil
		},
	}, existingBranch(), fakeTrainers{}, fakeOccupancy{count: func(context.Context, uuid.UUID) (int, error) {
		return 2, nil
	}}, stubTx{})
	if err := svc.Delete(context.Background(), uuid.New()); !errors.Is(err, ErrHasBookings) {
		t.Fatalf("Delete with bookings = %v, want ErrHasBookings", err)
	}
	if deleted {
		t.Fatal("repo.Delete called despite active bookings")
	}
}

func TestDeleteWithoutBookings(t *testing.T) {
	var deleted bool
	svc := NewService(fakeRepo{
		delete: func(_ context.Context, got uuid.UUID) error {
			deleted = true
			return nil
		},
	}, existingBranch(), fakeTrainers{}, fakeOccupancy{}, stubTx{})
	if err := svc.Delete(context.Background(), uuid.New()); err != nil {
		t.Fatalf("Delete clean: %v", err)
	}
	if !deleted {
		t.Fatal("repo.Delete not called for a class without bookings")
	}
}
