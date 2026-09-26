package memberships

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/members"
	"github.com/PandaX185/fitcore/internal/modules/packages"
	"github.com/PandaX185/fitcore/internal/paging"
)

// Service implements the membership lifecycle business rules on the ports.
type Service struct {
	repo     MembershipRepository
	packages PackageReader
	members  MemberReader
}

func NewService(repo MembershipRepository, packages PackageReader, members MemberReader) *Service {
	return &Service{repo: repo, packages: packages, members: members}
}

// Get returns the membership with the given ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Membership, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	return s.repo.GetByID(ctx, id)
}

// Create purchases a membership for a member from a package, starting now and
// expiring after the package's duration. A member may hold only one active
// membership at a time.
func (s *Service) Create(ctx context.Context, memberID, packageID, branchID uuid.UUID) (*Membership, error) {
	if memberID == uuid.Nil || packageID == uuid.Nil || branchID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	if _, err := s.members.Get(ctx, memberID); err != nil {
		if errors.Is(err, members.ErrNotFound) {
			return nil, ErrMemberNotFound
		}
		return nil, err
	}
	pkg, err := s.packages.Get(ctx, packageID)
	if err != nil {
		if errors.Is(err, packages.ErrNotFound) {
			return nil, ErrPackageNotFound
		}
		return nil, err
	}
	active, err := s.repo.HasActiveByMember(ctx, memberID)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, ErrDuplicateActive
	}
	now := time.Now().UTC()
	m := &Membership{
		ID:        uuid.New(),
		MemberID:  memberID,
		PackageID: packageID,
		BranchID:  branchID,
		StartsAt:  now,
		ExpiresAt: now.AddDate(0, 0, pkg.DurationDays),
		Status:    StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Update applies a partial patch (status lifecycle, expiry) and returns the
// refreshed membership.
func (s *Service) Update(ctx context.Context, id uuid.UUID, patch Patch) (*Membership, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if patch.Status != nil {
		switch *patch.Status {
		case StatusActive, StatusFrozen, StatusExpired:
		default:
			return nil, ErrInvalidInput
		}
	}
	if patch.ExpiresAt != nil {
		if patch.ExpiresAt.Before(existing.StartsAt) {
			return nil, ErrInvalidInput
		}
		if patch.Status != nil && *patch.Status == StatusExpired && patch.ExpiresAt.After(existing.ExpiresAt) {
			return nil, ErrInvalidInput
		}
	}
	if err := s.repo.Update(ctx, id, &patch); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// FindActiveByMemberAndBranch returns the member's live membership at a
// branch, mapping a missing row to ErrNotFound. Attendance consumes it to
// require an active membership before check-in.
func (s *Service) FindActiveByMemberAndBranch(ctx context.Context, memberID, branchID uuid.UUID) (*Membership, error) {
	return s.repo.FindActiveByMemberAndBranch(ctx, memberID, branchID)
}

// ListByMember returns a member's membership history, most recent first.
// ListByMember returns one page of a member's membership history ordered by
// (starts_on DESC, id), with an opaque cursor for the next page when more
// rows remain.
func (s *Service) ListByMember(ctx context.Context, memberID uuid.UUID, p MemberListParams) (*MemberListResult, error) {
	if memberID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	limit := paging.Limit(p.Limit)
	cursor, err := paging.DecodeCursor(p.Cursor)
	if err != nil {
		return nil, ErrInvalidInput
	}
	items, err := s.repo.ListByMember(ctx, &MemberListQuery{
		MemberID:      memberID,
		Limit:         limit + 1,
		AfterStartsAt: cursor.Key,
		AfterID:       cursor.ID,
	})
	if err != nil {
		return nil, err
	}
	res := &MemberListResult{Items: items}
	if len(items) > limit {
		res.Items = items[:limit]
		last := items[limit-1]
		res.NextCursor = paging.Cursor{Key: last.StartsAt.UTC().Format(time.RFC3339Nano), ID: last.ID}.Encode()
	}
	return res, nil
}
