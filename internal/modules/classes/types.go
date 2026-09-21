// Package classes models scheduled group classes delivered by trainers at
// branches.
package classes

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("class not found")

// Class is a scheduled group class.
type Class struct {
	ID        uuid.UUID
	BranchID  uuid.UUID
	TrainerID *uuid.UUID
	Name      string
	StartsAt  time.Time
	EndsAt    time.Time
	Capacity  int
	CreatedAt time.Time
	UpdatedAt time.Time
}
