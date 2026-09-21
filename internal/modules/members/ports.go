package members

import (
	"context"

	"github.com/google/uuid"
)

// MemberRepository is the persistence port consumed by the members service.
type MemberRepository interface {
	Create(ctx context.Context, m *Member) error
	GetByID(ctx context.Context, id uuid.UUID) (*Member, error)
	Update(ctx context.Context, m *Member) error
	List(ctx context.Context) ([]*Member, error)
}
