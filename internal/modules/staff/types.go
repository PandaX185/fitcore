// Package staff models FitCore's non-training employees.
package staff

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("staff member not found")

// Role describes a staff member's responsibilities.
type Role string

const (
	RoleManager   Role = "manager"
	RoleFrontDesk Role = "front_desk"
	RoleAdmin     Role = "admin"
)

// Staff is an employee of a branch.
type Staff struct {
	ID        uuid.UUID
	BranchID  uuid.UUID
	Name      string
	Email     string
	Phone     string
	Role      Role
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
