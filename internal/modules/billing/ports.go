package billing

import (
	"context"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/members"
	"github.com/PandaX185/fitcore/internal/modules/memberships"
)

// InvoiceRepository is the persistence port for invoice use cases.
type InvoiceRepository interface {
	Create(ctx context.Context, inv *Invoice) error
	GetByID(ctx context.Context, id uuid.UUID) (*Invoice, error)
	ListByMember(ctx context.Context, memberID uuid.UUID) ([]*Invoice, error)
	Update(ctx context.Context, id uuid.UUID, patch *Patch) error
}

// MemberReader is the slice of the members store billing needs.
type MemberReader interface {
	Get(ctx context.Context, id uuid.UUID) (*members.Member, error)
}

// MembershipReader is the slice of the memberships store billing needs to
// attach invoices to real memberships.
type MembershipReader interface {
	Get(ctx context.Context, id uuid.UUID) (*memberships.Membership, error)
}
