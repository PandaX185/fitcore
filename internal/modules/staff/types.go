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

// BranchListQuery is the paginated, branch-scoped query handed to
// persistence.
type BranchListQuery struct {
	// BranchID scopes the result to one branch.
	BranchID uuid.UUID
	// Limit is the maximum number of rows to return.
	Limit int
	// AfterName and AfterID form the exclusive cursor key into the
	// (name, id) ordering; AfterID is zero on the first page.
	AfterName string
	AfterID   uuid.UUID
}

// BranchListParams is the service-facing page request.
type BranchListParams struct {
	Limit  int
	Cursor string
}

// BranchListResult is a page of staff plus the cursor for the next page, if
// any.
type BranchListResult struct {
	Items      []*Staff
	NextCursor string
}
