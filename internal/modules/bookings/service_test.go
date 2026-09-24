package bookings

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/classes"
	"github.com/PandaX185/fitcore/internal/modules/members"
)

type fakeRepo struct {
	create             func(ctx context.Context, b *Booking) error
	getByID            func(ctx context.Context, id uuid.UUID) (*Booking, error)
	cancel             func(ctx context.Context, b *Booking) error
	listByClass        func(ctx context.Context, classID uuid.UUID) ([]*Booking, error)
	countActiveByClass func(ctx context.Context, classID uuid.UUID) (int, error)
}

func (f fakeRepo) Create(ctx context.Context, b *Booking) error {
	if f.create == nil {
		return nil
	}
	return f.create(ctx, b)
}
func (f fakeRepo) GetByID(ctx context.Context, id uuid.UUID) (*Booking, error) {
	return f.getByID(ctx, id)
}
func (f fakeRepo) Cancel(ctx context.Context, b *Booking) error {
	if f.cancel == nil {
		return nil
	}
	return f.cancel(ctx, b)
}
func (f fakeRepo) ListByClass(ctx context.Context, classID uuid.UUID) ([]*Booking, error) {
	if f.listByClass == nil {
		return nil, nil
	}
	return f.listByClass(ctx, classID)
}
func (f fakeRepo) CountActiveByClass(ctx context.Context, classID uuid.UUID) (int, error) {
	if f.countActiveByClass == nil {
		return 0, nil
	}
	return f.countActiveByClass(ctx, classID)
}

type fakeClasses struct {
	get func(ctx context.Context, id uuid.UUID) (*classes.Class, error)
}

func (f fakeClasses) Get(ctx context.Context, id uuid.UUID) (*classes.Class, error) {
	return f.get(ctx, id)
}

type fakeMembers struct {
	get func(ctx context.Context, id uuid.UUID) (*members.Member, error)
}

func (f fakeMembers) Get(ctx context.Context, id uuid.UUID) (*members.Member, error) {
	return f.get(ctx, id)
}

func futureClass(capacity int) fakeClasses {
	start := time.Now().UTC().Add(2 * time.Hour)
	return fakeClasses{get: func(_ context.Context, id uuid.UUID) (*classes.Class, error) {
		return &classes.Class{ID: id, Capacity: capacity, StartsAt: start, EndsAt: start.Add(time.Hour)}, nil
	}}
}

func TestGet(t *testing.T) {
	id := uuid.New()
	want := &Booking{ID: id}
	svc := NewService(fakeRepo{getByID: func(_ context.Context, got uuid.UUID) (*Booking, error) {
		if got != id {
			t.Fatalf("GetByID(%v)", got)
		}
		return want, nil
	}}, futureClass(10), fakeMembers{})
	got, err := svc.Get(context.Background(), id)
	if err != nil || got != want {
		t.Fatalf("Get = %v, %v", got, err)
	}
	if _, err := svc.Get(context.Background(), uuid.Nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Get nil = %v, want ErrInvalidInput", err)
	}
}

func TestCreate(t *testing.T) {
	classID, memberID := uuid.New(), uuid.New()
	var created *Booking
	svc := NewService(fakeRepo{create: func(_ context.Context, b *Booking) error {
		created = b
		return nil
	}}, futureClass(10), fakeMembers{get: func(_ context.Context, id uuid.UUID) (*members.Member, error) {
		return &members.Member{ID: id}, nil
	}})

	got, err := svc.Create(context.Background(), classID, memberID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID != created.ID || created.ClassID != classID || created.MemberID != memberID || created.Status != StatusBooked {
		t.Fatalf("booking = %+v", created)
	}
	if created.CancelledAt != nil {
		t.Fatalf("new booking has cancellation: %+v", created)
	}
}

func TestCreateInvalid(t *testing.T) {
	svc := NewService(fakeRepo{}, futureClass(10), fakeMembers{})
	id := uuid.New()
	if _, err := svc.Create(context.Background(), uuid.Nil, id); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Create nil class = %v", err)
	}
	if _, err := svc.Create(context.Background(), id, uuid.Nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Create nil member = %v", err)
	}
}

func TestCreateMemberNotFound(t *testing.T) {
	svc := NewService(fakeRepo{}, futureClass(10), fakeMembers{get: func(context.Context, uuid.UUID) (*members.Member, error) {
		return nil, members.ErrNotFound
	}})
	if _, err := svc.Create(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, ErrMemberNotFound) {
		t.Fatalf("Create = %v, want ErrMemberNotFound", err)
	}
}

func TestCreateClassNotFound(t *testing.T) {
	svc := NewService(fakeRepo{}, fakeClasses{get: func(context.Context, uuid.UUID) (*classes.Class, error) {
		return nil, classes.ErrNotFound
	}}, fakeMembers{get: func(_ context.Context, id uuid.UUID) (*members.Member, error) {
		return &members.Member{ID: id}, nil
	}})
	if _, err := svc.Create(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, ErrClassNotFound) {
		t.Fatalf("Create = %v, want ErrClassNotFound", err)
	}
}

func TestCreateClassFinished(t *testing.T) {
	past := time.Now().UTC().Add(-2 * time.Hour)
	svc := NewService(fakeRepo{}, fakeClasses{get: func(context.Context, uuid.UUID) (*classes.Class, error) {
		return &classes.Class{StartsAt: past, EndsAt: past.Add(-time.Hour)}, nil
	}}, fakeMembers{get: func(_ context.Context, id uuid.UUID) (*members.Member, error) {
		return &members.Member{ID: id}, nil
	}})
	if _, err := svc.Create(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Create finished class = %v, want ErrInvalidInput", err)
	}
}

func TestCreateClassFull(t *testing.T) {
	svc := NewService(fakeRepo{
		countActiveByClass: func(context.Context, uuid.UUID) (int, error) { return 10, nil },
	}, futureClass(10), fakeMembers{get: func(_ context.Context, id uuid.UUID) (*members.Member, error) {
		return &members.Member{ID: id}, nil
	}})
	if _, err := svc.Create(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, ErrClassFull) {
		t.Fatalf("Create full = %v, want ErrClassFull", err)
	}
}

func TestCreateDuplicate(t *testing.T) {
	svc := NewService(fakeRepo{create: func(context.Context, *Booking) error {
		return ErrDuplicate
	}}, futureClass(10), fakeMembers{get: func(_ context.Context, id uuid.UUID) (*members.Member, error) {
		return &members.Member{ID: id}, nil
	}})
	if _, err := svc.Create(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("Create dup = %v, want ErrDuplicate", err)
	}
}

func TestCancel(t *testing.T) {
	id := uuid.New()
	svc := NewService(fakeRepo{
		getByID: func(_ context.Context, got uuid.UUID) (*Booking, error) {
			return &Booking{ID: got, Status: StatusBooked}, nil
		},
		cancel: func(_ context.Context, b *Booking) error {
			if b.Status != StatusCancelled || b.CancelledAt == nil {
				t.Fatalf("cancel = %+v", b)
			}
			return nil
		},
	}, futureClass(10), fakeMembers{})
	got, err := svc.Cancel(context.Background(), id)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if got.Status != StatusCancelled || got.CancelledAt == nil {
		t.Fatalf("cancelled booking = %+v", got)
	}
}

func TestCancelAlreadyCancelledIsNoop(t *testing.T) {
	calls := 0
	svc := NewService(fakeRepo{
		getByID: func(_ context.Context, id uuid.UUID) (*Booking, error) {
			return &Booking{ID: id, Status: StatusCancelled}, nil
		},
		cancel: func(context.Context, *Booking) error {
			calls++
			return nil
		},
	}, futureClass(10), fakeMembers{})
	if _, err := svc.Cancel(context.Background(), uuid.New()); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if calls != 0 {
		t.Fatalf("repo.Cancel called %d times on already-cancelled", calls)
	}
}

func TestListByClass(t *testing.T) {
	classID := uuid.New()
	svc := NewService(fakeRepo{listByClass: func(_ context.Context, got uuid.UUID) ([]*Booking, error) {
		if got != classID {
			t.Fatalf("ListByClass(%v)", got)
		}
		return []*Booking{{ID: uuid.New()}}, nil
	}}, futureClass(10), fakeMembers{})
	got, err := svc.ListByClass(context.Background(), classID)
	if err != nil || len(got) != 1 {
		t.Fatalf("ListByClass = %v, %v", got, err)
	}
	if _, err := svc.ListByClass(context.Background(), uuid.Nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ListByClass nil = %v", err)
	}
}
