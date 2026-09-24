package attendance

import (
	"context"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/members"
	"github.com/PandaX185/fitcore/internal/modules/memberships"
)

// AttendanceRepository is the persistence port for attendance use cases.
type AttendanceRepository interface {
	Create(ctx context.Context, a *Attendance) error
	GetByID(ctx context.Context, id uuid.UUID) (*Attendance, error)
	ListByMember(ctx context.Context, memberID uuid.UUID) ([]*Attendance, error)
	// FindOpenByMember returns the member's open (checked-in) record, mapping
	// none to ErrNotFound.
	FindOpenByMember(ctx context.Context, memberID uuid.UUID) (*Attendance, error)
	// Close stamps checked_out_at on the record; ErrNotFound if already closed.
	Close(ctx context.Context, a *Attendance) error
}

// MemberReader is the slice of the members store attendance needs.
type MemberReader interface {
	Get(ctx context.Context, id uuid.UUID) (*members.Member, error)
}

// MembershipReader is the slice of the memberships store attendance needs to
// require an active membership at the branch.
type MembershipReader interface {
	FindActiveByMemberAndBranch(ctx context.Context, memberID, branchID uuid.UUID) (*memberships.Membership, error)
}
