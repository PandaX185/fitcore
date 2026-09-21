package classes

import (
	"context"

	"github.com/google/uuid"
)

// ClassRepository is the persistence port for classes.
type ClassRepository interface {
	Create(ctx context.Context, c *Class) error
	GetByID(ctx context.Context, id uuid.UUID) (*Class, error)
	ListByBranch(ctx context.Context, branchID uuid.UUID) ([]*Class, error)
	ListByTrainer(ctx context.Context, trainerID uuid.UUID) ([]*Class, error)
}
