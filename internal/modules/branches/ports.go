package branches

import (
	"context"

	"github.com/google/uuid"
)

// BranchRepository is the persistence port for branches.
type BranchRepository interface {
	Create(ctx context.Context, b *Branch) error
	GetByID(ctx context.Context, id uuid.UUID) (*Branch, error)
	List(ctx context.Context, q *ListQuery) ([]*Branch, error)
	Update(ctx context.Context, id uuid.UUID, patch *Patch) error
}
