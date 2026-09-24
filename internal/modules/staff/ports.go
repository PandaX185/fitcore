package staff

import (
	"context"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/branches"
)

// StaffRepository is the persistence port for staff use cases.
type StaffRepository interface {
	Create(ctx context.Context, s *Staff) error
	GetByID(ctx context.Context, id uuid.UUID) (*Staff, error)
	ListByBranch(ctx context.Context, branchID uuid.UUID) ([]*Staff, error)
	Update(ctx context.Context, id uuid.UUID, patch *Patch) error
}

// BranchReader is the slice of the branches store staff needs to reject
// assignments to unknown branches.
type BranchReader interface {
	Get(ctx context.Context, id uuid.UUID) (*branches.Branch, error)
}
