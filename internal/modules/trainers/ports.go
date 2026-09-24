package trainers

import (
	"context"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/branches"
)

// TrainerRepository is the persistence port for trainer use cases.
type TrainerRepository interface {
	Create(ctx context.Context, t *Trainer) error
	GetByID(ctx context.Context, id uuid.UUID) (*Trainer, error)
	ListByBranch(ctx context.Context, branchID uuid.UUID) ([]*Trainer, error)
	Update(ctx context.Context, id uuid.UUID, patch *Patch) error
}

// BranchReader is the slice of the branches store trainers needs to reject
// assignments to unknown branches.
type BranchReader interface {
	Get(ctx context.Context, id uuid.UUID) (*branches.Branch, error)
}
