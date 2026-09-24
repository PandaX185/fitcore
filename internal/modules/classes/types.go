// Package classes models scheduled group classes delivered by trainers at
// branches.
package classes

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound        = errors.New("class not found")
	ErrInvalidInput    = errors.New("invalid class input")
	ErrBranchNotFound  = errors.New("branch not found")
	ErrTrainerNotFound = errors.New("trainer not found")
)

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

// Patch is a partial update. Nil fields are left unchanged.
type Patch struct {
	TrainerID *uuid.UUID
	Name      *string
	StartsAt  *time.Time
	EndsAt    *time.Time
	Capacity  *int
}
