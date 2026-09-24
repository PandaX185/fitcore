package attendance

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/members"
	"github.com/PandaX185/fitcore/internal/modules/memberships"
)

type fakeRepo struct {
	create           func(ctx context.Context, a *Attendance) error
	getByID          func(ctx context.Context, id uuid.UUID) (*Attendance, error)
	listByMember     func(ctx context.Context, memberID uuid.UUID) ([]*Attendance, error)
	findOpenByMember func(ctx context.Context, memberID uuid.UUID) (*Attendance, error)
	close            func(ctx context.Context, a *Attendance) error
}

func (f fakeRepo) Create(ctx context.Context, a *Attendance) error {
	if f.create == nil {
		return nil
	}
	return f.create(ctx, a)
}
func (f fakeRepo) GetByID(ctx context.Context, id uuid.UUID) (*Attendance, error) {
	return f.getByID(ctx, id)
}
func (f fakeRepo) ListByMember(ctx context.Context, memberID uuid.UUID) ([]*Attendance, error) {
	if f.listByMember == nil {
		return nil, nil
	}
	return f.listByMember(ctx, memberID)
}
func (f fakeRepo) FindOpenByMember(ctx context.Context, memberID uuid.UUID) (*Attendance, error) {
	if f.findOpenByMember == nil {
		return nil, ErrNotFound
	}
	return f.findOpenByMember(ctx, memberID)
}
func (f fakeRepo) Close(ctx context.Context, a *Attendance) error {
	if f.close == nil {
		return nil
	}
	return f.close(ctx, a)
}

type fakeMembers struct {
	get func(ctx context.Context, id uuid.UUID) (*members.Member, error)
}

func (f fakeMembers) Get(ctx context.Context, id uuid.UUID) (*members.Member, error) {
	return f.get(ctx, id)
}

type fakeMemberships struct {
	findActive func(ctx context.Context, memberID, branchID uuid.UUID) (*memberships.Membership, error)
}

func (f fakeMemberships) FindActiveByMemberAndBranch(ctx context.Context, memberID, branchID uuid.UUID) (*memberships.Membership, error) {
	return f.findActive(ctx, memberID, branchID)
}

func activeMembershipIn(branchID uuid.UUID) fakeMemberships {
	return fakeMemberships{findActive: func(_ context.Context, memberID, b uuid.UUID) (*memberships.Membership, error) {
		if b != branchID {
			return nil, memberships.ErrNotFound
		}
		return &memberships.Membership{ID: uuid.New(), MemberID: memberID, BranchID: b}, nil
	}}
}

func TestCheckIn(t *testing.T) {
	memberID, branchID := uuid.New(), uuid.New()
	var created *Attendance
	svc := NewService(fakeRepo{create: func(_ context.Context, a *Attendance) error {
		created = a
		return nil
	}}, fakeMembers{get: func(_ context.Context, id uuid.UUID) (*members.Member, error) {
		return &members.Member{ID: id}, nil
	}}, activeMembershipIn(branchID))

	got, err := svc.CheckIn(context.Background(), memberID, branchID)
	if err != nil {
		t.Fatalf("CheckIn: %v", err)
	}
	if got.ID != created.ID || created.MemberID != memberID || created.BranchID != branchID {
		t.Fatalf("attendance = %+v", created)
	}
	if created.MembershipID == uuid.Nil {
		t.Fatalf("membership id not recorded: %+v", created)
	}
}

func TestCheckInInvalid(t *testing.T) {
	svc := NewService(fakeRepo{}, fakeMembers{}, activeMembershipIn(uuid.New()))
	id := uuid.New()
	if _, err := svc.CheckIn(context.Background(), uuid.Nil, id); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CheckIn nil member = %v", err)
	}
	if _, err := svc.CheckIn(context.Background(), id, uuid.Nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CheckIn nil branch = %v", err)
	}
}

func TestCheckInMemberNotFound(t *testing.T) {
	svc := NewService(fakeRepo{}, fakeMembers{get: func(context.Context, uuid.UUID) (*members.Member, error) {
		return nil, members.ErrNotFound
	}}, activeMembershipIn(uuid.New()))
	if _, err := svc.CheckIn(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, ErrMemberNotFound) {
		t.Fatalf("CheckIn = %v, want ErrMemberNotFound", err)
	}
}

func TestCheckInNoActiveMembership(t *testing.T) {
	svc := NewService(fakeRepo{}, fakeMembers{get: func(_ context.Context, id uuid.UUID) (*members.Member, error) {
		return &members.Member{ID: id}, nil
	}}, fakeMemberships{findActive: func(context.Context, uuid.UUID, uuid.UUID) (*memberships.Membership, error) {
		return nil, memberships.ErrNotFound
	}})
	if _, err := svc.CheckIn(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, ErrNoActiveMembership) {
		t.Fatalf("CheckIn = %v, want ErrNoActiveMembership", err)
	}
}

func TestCheckInAlreadyOpen(t *testing.T) {
	branchID := uuid.New()
	svc := NewService(fakeRepo{
		findOpenByMember: func(context.Context, uuid.UUID) (*Attendance, error) {
			return &Attendance{ID: uuid.New()}, nil
		},
	}, fakeMembers{get: func(_ context.Context, id uuid.UUID) (*members.Member, error) {
		return &members.Member{ID: id}, nil
	}}, activeMembershipIn(branchID))
	if _, err := svc.CheckIn(context.Background(), uuid.New(), branchID); !errors.Is(err, ErrAlreadyCheckedIn) {
		t.Fatalf("CheckIn = %v, want ErrAlreadyCheckedIn", err)
	}
}

func TestCheckOut(t *testing.T) {
	memberID := uuid.New()
	open := &Attendance{ID: uuid.New(), MemberID: memberID}
	svc := NewService(fakeRepo{
		findOpenByMember: func(_ context.Context, id uuid.UUID) (*Attendance, error) {
			return open, nil
		},
		close: func(_ context.Context, a *Attendance) error {
			if a.CheckedOutAt == nil {
				t.Fatalf("close without timestamp: %+v", a)
			}
			return nil
		},
	}, fakeMembers{}, fakeMemberships{})
	got, err := svc.CheckOut(context.Background(), memberID)
	if err != nil {
		t.Fatalf("CheckOut: %v", err)
	}
	if got.CheckedOutAt == nil {
		t.Fatalf("CheckOut left timestamp nil: %+v", got)
	}
}

func TestCheckOutNoOpenRecord(t *testing.T) {
	svc := NewService(fakeRepo{}, fakeMembers{}, fakeMemberships{})
	if _, err := svc.CheckOut(context.Background(), uuid.New()); !errors.Is(err, ErrNoOpenRecord) {
		t.Fatalf("CheckOut = %v, want ErrNoOpenRecord", err)
	}
	if _, err := svc.CheckOut(context.Background(), uuid.Nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CheckOut nil member = %v, want ErrInvalidInput", err)
	}
}

func TestListByMember(t *testing.T) {
	memberID := uuid.New()
	svc := NewService(fakeRepo{listByMember: func(_ context.Context, got uuid.UUID) ([]*Attendance, error) {
		if got != memberID {
			t.Fatalf("ListByMember(%v)", got)
		}
		return []*Attendance{{ID: uuid.New()}}, nil
	}}, fakeMembers{}, fakeMemberships{})
	got, err := svc.ListByMember(context.Background(), memberID)
	if err != nil || len(got) != 1 {
		t.Fatalf("ListByMember = %v, %v", got, err)
	}
	if _, err := svc.ListByMember(context.Background(), uuid.Nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ListByMember nil = %v", err)
	}
}
