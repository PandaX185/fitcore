package attendance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/members"
	"github.com/PandaX185/fitcore/internal/modules/memberships"
	"github.com/PandaX185/fitcore/internal/paging"
)

type fakeRepo struct {
	create           func(ctx context.Context, a *Attendance) error
	getByID          func(ctx context.Context, id uuid.UUID) (*Attendance, error)
	listByMember     func(ctx context.Context, q *MemberListQuery) ([]*Attendance, error)
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
func (f fakeRepo) ListByMember(ctx context.Context, q *MemberListQuery) ([]*Attendance, error) {
	if f.listByMember == nil {
		return nil, nil
	}
	return f.listByMember(ctx, q)
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
	svc := NewService(fakeRepo{listByMember: func(_ context.Context, q *MemberListQuery) ([]*Attendance, error) {
		if q.MemberID != memberID {
			t.Fatalf("MemberListQuery(%+v)", q)
		}
		return []*Attendance{{ID: uuid.New()}}, nil
	}}, fakeMembers{}, fakeMemberships{})
	res, err := svc.ListByMember(context.Background(), memberID, MemberListParams{})
	if err != nil || len(res.Items) != 1 {
		t.Fatalf("ListByMember = %v, %v", res, err)
	}
	if _, err := svc.ListByMember(context.Background(), uuid.Nil, MemberListParams{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ListByMember nil = %v", err)
	}
}

func TestListByMemberFirstPage(t *testing.T) {
	memberID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)
	svc := NewService(fakeRepo{
		listByMember: func(_ context.Context, q *MemberListQuery) ([]*Attendance, error) {
			if q.Limit != paging.DefaultLimit+1 {
				t.Fatalf("Limit = %d, want %d", q.Limit, paging.DefaultLimit+1)
			}
			if q.AfterCheckedInAt != "" || q.AfterID != uuid.Nil {
				t.Fatalf("cursor start = %q/%v, want empty", q.AfterCheckedInAt, q.AfterID)
			}
			items := []*Attendance{}
			for i := 0; i < paging.DefaultLimit+1; i++ {
				items = append(items, &Attendance{ID: uuid.New(), CheckedInAt: now})
			}
			return items, nil
		},
	}, fakeMembers{}, fakeMemberships{})

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
		listByMember: func(context.Context, *MemberListQuery) ([]*Attendance, error) {
			return []*Attendance{{ID: uuid.New()}}, nil
		},
	}, fakeMembers{}, fakeMemberships{})

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
				listByMember: func(_ context.Context, q *MemberListQuery) ([]*Attendance, error) {
					if q.Limit != tt.want+1 {
						t.Fatalf("repo Limit = %d, want %d", q.Limit, tt.want+1)
					}
					return nil, nil
				},
			}, fakeMembers{}, fakeMemberships{})
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
		listByMember: func(_ context.Context, q *MemberListQuery) ([]*Attendance, error) {
			if q.AfterCheckedInAt != at.Format(time.RFC3339Nano) {
				t.Fatalf("AfterCheckedInAt = %q, want %q", q.AfterCheckedInAt, at.Format(time.RFC3339Nano))
			}
			if q.AfterID != id {
				t.Fatalf("AfterID = %v, want %v", q.AfterID, id)
			}
			return nil, nil
		},
	}, fakeMembers{}, fakeMemberships{})

	if _, err := svc.ListByMember(context.Background(), uuid.New(), MemberListParams{Cursor: cursor}); err != nil {
		t.Fatalf("ListByMember: unexpected error %v", err)
	}
}

func TestListByMemberRejectsBadCursor(t *testing.T) {
	for _, cur := range []string{"%%%", "not-base64-!", "eyJuYW1lIjoibiJ9"} {
		t.Run(cur, func(t *testing.T) {
			svc := NewService(fakeRepo{
				listByMember: func(context.Context, *MemberListQuery) ([]*Attendance, error) {
					t.Fatal("repo must not be called for a bad cursor")
					return nil, nil
				},
			}, fakeMembers{}, fakeMemberships{})
			_, err := svc.ListByMember(context.Background(), uuid.New(), MemberListParams{Cursor: cur})
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("ListByMember error = %v, want ErrInvalidInput", err)
			}
		})
	}
}
