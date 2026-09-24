package memberships

import (
	"context"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/members"
	"github.com/PandaX185/fitcore/internal/modules/packages"
)

// MembershipRepository is the persistence port for membership use cases.
type MembershipRepository interface {
	Create(ctx context.Context, m *Membership) error
	GetByID(ctx context.Context, id uuid.UUID) (*Membership, error)
	ListByMember(ctx context.Context, memberID uuid.UUID) ([]*Membership, error)
	Update(ctx context.Context, id uuid.UUID, patch *Patch) error
	// HasActiveByMember reports whether the member holds a live membership;
	// the store's partial unique index backs the same invariant at rest.
	HasActiveByMember(ctx context.Context, memberID uuid.UUID) (bool, error)
	// FindActiveByMemberAndBranch returns the member's live membership at a
	// branch, mapping none to ErrNotFound. Attendance consumes it.
	FindActiveByMemberAndBranch(ctx context.Context, memberID, branchID uuid.UUID) (*Membership, error)
}

// PackageReader is the slice of the packages store memberships needs to
// derive expiry from the purchased package's duration.
type PackageReader interface {
	Get(ctx context.Context, id uuid.UUID) (*packages.Package, error)
}

// MemberReader is the slice of the members store memberships needs to reject
// purchases for unknown members.
type MemberReader interface {
	Get(ctx context.Context, id uuid.UUID) (*members.Member, error)
}
