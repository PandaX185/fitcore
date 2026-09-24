package bookings

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/classes"
	"github.com/PandaX185/fitcore/internal/modules/members"
)

// Service implements the class-booking business rules on the ports.
type Service struct {
	repo    BookingRepository
	classes ClassReader
	members MemberReader
}

func NewService(repo BookingRepository, classes ClassReader, members MemberReader) *Service {
	return &Service{repo: repo, classes: classes, members: members}
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
	cl, err := s.classes.Get(ctx, classID)
	if err != nil {
		if errors.Is(err, classes.ErrNotFound) {
			return nil, ErrClassNotFound
		}
		return nil, err
	}
	now := time.Now().UTC()
	if !cl.EndsAt.After(now) {
		return nil, ErrInvalidInput
	}
	count, err := s.repo.CountActiveByClass(ctx, classID)
	if err != nil {
		return nil, err
	}
	if count >= cl.Capacity {
		return nil, ErrClassFull
	}
	b := &Booking{
		ID:        uuid.New(),
		ClassID:   classID,
		MemberID:  memberID,
		Status:    StatusBooked,
		BookedAt:  now,
		CreatedAt: now,
	}
	if err := s.repo.Create(ctx, b); err != nil {
		return nil, err
	}
	return b, nil
}

// Cancel moves a booked seat to cancelled, stamping cancelled_at. Cancelling
// an already-cancelled booking is a no-op.
func (s *Service) Cancel(ctx context.Context, id uuid.UUID) (*Booking, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Status == StatusCancelled {
		return existing, nil
	}
	cancelledAt := time.Now().UTC()
	existing.Status = StatusCancelled
	existing.CancelledAt = &cancelledAt
	if err := s.repo.Cancel(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

// ListByClass returns the bookings for a class, for roll-call rendering.
func (s *Service) ListByClass(ctx context.Context, classID uuid.UUID) ([]*Booking, error) {
	if classID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	return s.repo.ListByClass(ctx, classID)
}
