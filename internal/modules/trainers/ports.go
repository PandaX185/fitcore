package trainers

import (
	"context"

	"github.com/google/uuid"
)

// TrainerRepository is the persistence port for trainers.
type TrainerRepository interface {
	Create(ctx context.Context, t *Trainer) error
	GetByID(ctx context.Context, id uuid.UUID) (*Trainer, error)
	GetByEmail(ctx context.Context, email string) (*Trainer, error)
	ListByBranch(ctx context.Context, branchID uuid.UUID) ([]*Trainer, error)
}
