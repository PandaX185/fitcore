// Package members implements the member lifecycle use cases of FitCore.
package members

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound       = errors.New("member not found")
	ErrDuplicateEmail = errors.New("email already in use")
	ErrInvalidInput   = errors.New("invalid member input")
)

// Status describes the lifecycle state of a member.
type Status string

const (
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
)

// Member is the application-facing member aggregate.
type Member struct {
	ID        uuid.UUID
	BranchID  uuid.UUID
	Name      string
	Email     string
	Phone     string
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Patch is a partial update. Nil fields are left unchanged.
type Patch struct {
	Name   *string
	Email  *string
	Phone  *string
	Status *Status
}

// ListQuery is the paginated query handed to persistence.
type ListQuery struct {
	// Limit is the maximum number of rows to return.
	Limit int
	// AfterName and AfterID form the exclusive cursor key into the
	// (name, id) ordering; both zero-valued on the first page.
	AfterName string
	AfterID   uuid.UUID
}

// ListParams is the service-facing page request.
type ListParams struct {
	Limit  int
	Cursor string
}

// ListResult is a page of members plus the cursor for the next page, if any.
type ListResult struct {
	Items      []*Member
	NextCursor string
}
