package memberships

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/branches"
	"github.com/PandaX185/fitcore/internal/modules/members"
	"github.com/PandaX185/fitcore/internal/modules/packages"
	"github.com/PandaX185/fitcore/internal/paging"
	"github.com/PandaX185/fitcore/internal/transact"
)

// Service implements the membership lifecycle business rules on the ports.
type Service struct {
	repo     MembershipRepository
	packages PackageReader
	members  MemberReader
	branches BranchReader
	tx       transact.Transactor
}

func NewService(repo MembershipRepository, packages PackageReader, members MemberReader, branches BranchReader, tx transact.Transactor) *Service {
	return &Service{repo: repo, packages: packages, members: members, branches: branches, tx: tx}
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
// membership at a time. The branch must exist; a foreign-key violation on
// insert is mapped defensively to ErrBranchNotFound for stores that skip the
// pre-read.
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
	if _, err := s.branches.Get(ctx, branchID); err != nil {
		if errors.Is(err, branches.ErrNotFound) {
			return nil, ErrBranchNotFound
		}
		return nil, err
	}
	var m *Membership
	err = s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		active, err := s.repo.HasActiveByMember(txCtx, memberID)
		if err != nil {
			return err
		}
		if active {
			return ErrDuplicateActive
		}
		now := time.Now().UTC()
		m = &Membership{
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
		if err := s.repo.Create(txCtx, m); err != nil {
			if isForeignKeyViolation(err) {
				return ErrBranchNotFound
			}
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return m, nil
}

// isForeignKeyViolation reports a relational foreign-key breach without
// importing the driver: it matches the SQLSTATE class and the usual message
// text.
func isForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23503") ||
		strings.Contains(msg, "violates foreign key") ||
		strings.Contains(msg, "foreign key constraint")
}

// Update applies a partial patch (status lifecycle, expiry) and returns the
// refreshed membership. The lifecycle is a strict state machine:
// active may move to frozen or expired, frozen may return to active or move
// to expired, and expired is terminal — anything else is ErrInvalidInput.
// The write is conditional on the observed status (lost updates surface as
// ErrNotFound from the store instead of silently overwriting a concurrent
// move).
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
		if *patch.Status != existing.Status && !validTransition(existing.Status, *patch.Status) {
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
	var updated *Membership
	err = s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repo.UpdateStatus(txCtx, id, existing.Status, &patch); err != nil {
			return err
		}
		u, err := s.repo.GetByID(txCtx, id)
		if err != nil {
			return err
		}
		updated = u
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// validTransition reports whether a status change follows the membership
// lifecycle. Same-status writes are allowed (idempotent no-ops).
func validTransition(from, to Status) bool {
	switch from {
	case StatusActive:
		return to == StatusFrozen || to == StatusExpired
	case StatusFrozen:
		return to == StatusActive || to == StatusExpired
	default:
		return false
	}
}

// FindActiveByMemberAndBranch returns the member's live membership at a
// branch, mapping a missing row to ErrNotFound. Attendance consumes it to
// require an active membership before check-in.
//
// Liveness is evaluated at read time (status active AND not yet expired):
// there is no background expirer, so a membership whose time has passed
// keeps its stored status until explicitly transitioned to expired, but it
// no longer counts as active here.
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
	if cursor.Key != "" {
		if _, err := time.Parse(time.RFC3339Nano, cursor.Key); err != nil {
			return nil, ErrInvalidInput
		}
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
