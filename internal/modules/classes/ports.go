package classes

import (
	"context"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/branches"
	"github.com/PandaX185/fitcore/internal/modules/trainers"
)

// ClassRepository is the persistence port for classes.
type ClassRepository interface {
	Create(ctx context.Context, c *Class) error
	GetByID(ctx context.Context, id uuid.UUID) (*Class, error)
	// GetForUpdate returns the class row locked (SELECT ... FOR UPDATE) so
	// capacity checks and the writes that follow serialize inside one
	// transaction.
	GetForUpdate(ctx context.Context, id uuid.UUID) (*Class, error)
	Update(ctx context.Context, id uuid.UUID, patch *Patch) error
	Delete(ctx context.Context, id uuid.UUID) error
	// List supports filtering by branch and/or trainer. Nil filters mean the
	// corresponding constraint is omitted.
	List(ctx context.Context, q *ListQuery) ([]*Class, error)
}

// BranchReader is the slice of the branches store classes needs.
type BranchReader interface {
	Get(ctx context.Context, id uuid.UUID) (*branches.Branch, error)
}

// TrainerReader is the slice of the trainers store classes needs.
type TrainerReader interface {
	Get(ctx context.Context, id uuid.UUID) (*trainers.Trainer, error)
}

// ClassOccupancyReader is the slice of the bookings store classes needs to
// guard capacity shrinks and deletes against live bookings.
type ClassOccupancyReader interface {
	CountActiveByClass(ctx context.Context, classID uuid.UUID) (int, error)
}
