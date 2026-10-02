package bookings

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/classes"
	"github.com/PandaX185/fitcore/internal/modules/members"
	"github.com/PandaX185/fitcore/internal/paging"
	"github.com/PandaX185/fitcore/internal/transact"
)

// Service implements the class-booking business rules on the ports.
type Service struct {
	repo    BookingRepository
	classes ClassReader
	members MemberReader
	tx      transact.Transactor
}

func NewService(repo BookingRepository, classes ClassReader, members MemberReader, tx transact.Transactor) *Service {
	return &Service{repo: repo, classes: classes, members: members, tx: tx}
}

// Get returns the booking with the given ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Booking, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	return s.repo.GetByID(ctx, id)
}

// Create reserves a seat for the class. Refusals: unknown class or member
// (404), the class already running (409), the member already holds an active
// booking (409), and the class at capacity (409).
//
// The locked class read, the occupancy count, and the insert run inside one
// transaction: the class row is selected FOR UPDATE first, so concurrent
// bookers serialize and the capacity check cannot over-admit (TOCTOU).
func (s *Service) Create(ctx context.Context, classID, memberID uuid.UUID) (*Booking, error) {
	if classID == uuid.Nil || memberID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	if _, err := s.members.Get(ctx, memberID); err != nil {
		if errors.Is(err, members.ErrNotFound) {
			return nil, ErrMemberNotFound
		}
		return nil, err
	}
	var b *Booking
	err := s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		cl, err := s.classes.GetForUpdate(txCtx, classID)
		if err != nil {
			if errors.Is(err, classes.ErrNotFound) {
				return ErrClassNotFound
			}
			return err
		}
		now := time.Now().UTC()
		if !cl.EndsAt.After(now) {
			return ErrInvalidInput
		}
		count, err := s.repo.CountActiveByClass(txCtx, classID)
		if err != nil {
			return err
		}
		if count >= cl.Capacity {
			return ErrClassFull
		}
		b = &Booking{
			ID:        uuid.New(),
			ClassID:   classID,
			MemberID:  memberID,
			Status:    StatusBooked,
			BookedAt:  now,
			CreatedAt: now,
		}
		if err := s.repo.Create(txCtx, b); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Cancel moves a booked seat to cancelled, stamping cancelled_at. The store
// flips the row only while it is still booked (UPDATE ... WHERE booked), so
// concurrent cancels serialize: the loser re-reads the row and returns the
// already-cancelled booking (200) instead of failing. A missing row maps to
// ErrNotFound.
func (s *Service) Cancel(ctx context.Context, id uuid.UUID) (*Booking, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	cancelledAt := time.Now().UTC()
	b := &Booking{ID: id, Status: StatusCancelled, CancelledAt: &cancelledAt}
	if err := s.repo.Cancel(ctx, b); err != nil {
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		existing, getErr := s.repo.GetByID(ctx, id)
		if getErr != nil {
			return nil, getErr
		}
		return existing, nil
	}
	return s.repo.GetByID(ctx, id)
}

// ListByClass returns one page of bookings for a class ordered by
// (created_at, id), for roll-call rendering, with an opaque cursor for the
// next page when more rows remain.
func (s *Service) ListByClass(ctx context.Context, classID uuid.UUID, p ClassListParams) (*ClassListResult, error) {
	if classID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	limit := paging.Limit(p.Limit)
	cursor, err := paging.DecodeCursor(p.Cursor)
	if err != nil {
		return nil, ErrInvalidInput
	}
	if cursor.Key != "" {
		if _, err := time.Parse(time.RFC3339Nano, cursor.Key); err != nil {
			return nil, ErrInvalidInput
		}
	}
	items, err := s.repo.ListByClass(ctx, &ClassListQuery{
		ClassID:       classID,
		Limit:         limit + 1,
		AfterBookedAt: cursor.Key,
		AfterID:       cursor.ID,
	})
	if err != nil {
		return nil, err
	}
	res := &ClassListResult{Items: items}
	if len(items) > limit {
		res.Items = items[:limit]
		last := items[limit-1]
		res.NextCursor = paging.Cursor{Key: last.BookedAt.UTC().Format(time.RFC3339Nano), ID: last.ID}.Encode()
	}
	return res, nil
}
