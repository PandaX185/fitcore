// Package staff models FitCore's non-training employees.
package staff

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("staff member not found")

// Staff is an employee of a branch.
type Staff struct {
	ID          uuid.UUID
	BranchID    uuid.UUID
	Name        string
	Email       string
	Phone       string
	Permissions []string
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
