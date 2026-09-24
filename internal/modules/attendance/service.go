package attendance

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/members"
	"github.com/PandaX185/fitcore/internal/modules/memberships"
)

// Service implements the attendance business rules on the ports.
type Service struct {
	repo        AttendanceRepository
	members     MemberReader
	memberships MembershipReader
}

func NewService(repo AttendanceRepository, members MemberReader, memberships MembershipReader) *Service {
	return &Service{repo: repo, members: members, memberships: memberships}
}

// Get returns the attendance record with the given ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Attendance, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	return s.repo.GetByID(ctx, id)
}

// CheckIn opens a visit for a member at a branch. The member must exist and
// hold an active membership at that branch; a member cannot hold two open
// visits.
func (s *Service) CheckIn(ctx context.Context, memberID, branchID uuid.UUID) (*Attendance, error) {
	if memberID == uuid.Nil || branchID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	if _, err := s.members.Get(ctx, memberID); err != nil {
		if errors.Is(err, members.ErrNotFound) {
			return nil, ErrMemberNotFound
		}
		return nil, err
	}
	membership, err := s.memberships.FindActiveByMemberAndBranch(ctx, memberID, branchID)
	if err != nil {
		if errors.Is(err, memberships.ErrNotFound) {
			return nil, ErrNoActiveMembership
		}
		return nil, err
	}
	now := time.Now().UTC()
	if open, err := s.repo.FindOpenByMember(ctx, memberID); err == nil && open != nil {
		return nil, ErrAlreadyCheckedIn
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	a := &Attendance{
		ID:           uuid.New(),
		MemberID:     memberID,
		BranchID:     branchID,
		MembershipID: membership.ID,
		CheckedInAt:  now,
		CreatedAt:    now,
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// CheckOut closes the member's open visit.
func (s *Service) CheckOut(ctx context.Context, memberID uuid.UUID) (*Attendance, error) {
	if memberID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	open, err := s.repo.FindOpenByMember(ctx, memberID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNoOpenRecord
		}
		return nil, err
	}
	if open.CheckedOutAt != nil {
		return nil, ErrAlreadyCheckedOut
	}
	now := time.Now().UTC()
	open.CheckedOutAt = &now
	if err := s.repo.Close(ctx, open); err != nil {
		return nil, err
	}
	return open, nil
}

// ListByMember returns a member's visit history, most recent first.
func (s *Service) ListByMember(ctx context.Context, memberID uuid.UUID) ([]*Attendance, error) {
	if memberID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	return s.repo.ListByMember(ctx, memberID)
}
