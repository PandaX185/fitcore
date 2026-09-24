// Package staff models the gym's staff members and their grants.
package staff

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/auth"
)

var (
	ErrNotFound       = errors.New("staff not found")
	ErrInvalidInput   = errors.New("invalid staff input")
	ErrDuplicateEmail = errors.New("email already in use")
	ErrBranchNotFound = errors.New("branch not found")
)

// Staff is a staff member and their grant set.
type Staff struct {
	ID          uuid.UUID
	BranchID    uuid.UUID
	Name        string
	Email       string
	Phone       string
	Permissions []auth.Permission
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Patch is a partial update. Nil fields are left unchanged.
type Patch struct {
	Name        *string
	Email       *string
	Phone       *string
	Permissions *[]auth.Permission
	Active      *bool
}
