package staff

import (
	"context"

	"github.com/google/uuid"
)

// StaffRepository is the persistence port for staff members.
type StaffRepository interface {
	Create(ctx context.Context, s *Staff) error
	GetByID(ctx context.Context, id uuid.UUID) (*Staff, error)
	ListByBranch(ctx context.Context, branchID uuid.UUID) ([]*Staff, error)
}
