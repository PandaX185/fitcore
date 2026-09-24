// Package trainers models the gym's personal trainers.
package trainers

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound       = errors.New("trainer not found")
	ErrInvalidInput   = errors.New("invalid trainer input")
	ErrDuplicateEmail = errors.New("email already in use")
	ErrBranchNotFound = errors.New("branch not found")
)

// Trainer is a trainer.
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

// Patch is a partial update. Nil fields are left unchanged.
type Patch struct {
	Name   *string
	Email  *string
	Phone  *string
	Active *bool
}
