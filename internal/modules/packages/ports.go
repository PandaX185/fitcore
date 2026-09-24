package packages

import (
	"context"

	"github.com/google/uuid"
)

// PackageRepository is the persistence port for membership packages.
type PackageRepository interface {
	Create(ctx context.Context, p *Package) error
	GetByID(ctx context.Context, id uuid.UUID) (*Package, error)
	List(ctx context.Context) ([]*Package, error)
	Update(ctx context.Context, id uuid.UUID, patch *Patch) error
}
