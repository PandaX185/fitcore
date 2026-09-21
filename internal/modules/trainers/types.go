// Package trainers models FitCore's trainers who teach classes and coach
// members.
package trainers

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("trainer not found")

// Trainer is a fitness professional attached to a branch.
type Trainer struct {
	ID        uuid.UUID
	BranchID  uuid.UUID
	Name      string
	Email     string
	Phone     string
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
