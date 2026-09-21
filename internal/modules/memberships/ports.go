package memberships

import (
	"context"

	"github.com/google/uuid"
)

// MembershipRepository is the persistence port for membership use cases.
type MembershipRepository interface {
	Create(ctx context.Context, m *Membership) error
	GetByID(ctx context.Context, id uuid.UUID) (*Membership, error)
	ListByMember(ctx context.Context, memberID uuid.UUID) ([]*Membership, error)
	Update(ctx context.Context, m *Membership) error
}

// PackageRepository is the persistence port for membership packages.
type PackageRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Package, error)
}
