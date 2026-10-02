package members

import (
	"context"

	"github.com/google/uuid"
)

// MemberRepository is the persistence port consumed by the members service.
type MemberRepository interface {
	Create(ctx context.Context, m *Member) error
	GetByID(ctx context.Context, id uuid.UUID) (*Member, error)
	Update(ctx context.Context, id uuid.UUID, patch *Patch) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, q *ListQuery) ([]*Member, error)
}

// ActiveMembershipChecker reports whether the member holds a live
// membership; the memberships store backs it.
type ActiveMembershipChecker interface {
	HasActiveByMember(ctx context.Context, memberID uuid.UUID) (bool, error)
}

// PendingInvoiceChecker reports whether the member owes a pending invoice;
// the billing store backs it.
type PendingInvoiceChecker interface {
	HasPendingByMember(ctx context.Context, memberID uuid.UUID) (bool, error)
}

// OpenAttendanceChecker reports whether the member has an open visit. It is
// backed by the attendance store's open-visit lookup, where a missing row
// (ErrNotFound) means no open visit. The attendance domain type is
// deliberately not referenced here so members stays importable from
// attendance (no cycle).
type OpenAttendanceChecker interface {
	HasOpenByMember(ctx context.Context, memberID uuid.UUID) (bool, error)
}
