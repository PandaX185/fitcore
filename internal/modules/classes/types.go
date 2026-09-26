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

// ListQuery is the paginated, filterable query handed to persistence.
type ListQuery struct {
	// BranchID and TrainerID narrow the result; nil means unfiltered.
	BranchID  *uuid.UUID
	TrainerID *uuid.UUID
	// Limit is the maximum number of rows to return.
	Limit int
	// AfterStartsAt and AfterID form the exclusive cursor key into the
	// (starts_at, id) ordering; AfterID is zero on the first page.
	// AfterStartsAt carries an RFC3339 timestamp as text.
	AfterStartsAt string
	AfterID       uuid.UUID
}

// ListParams is the service-facing page request.
type ListParams struct {
	BranchID  *uuid.UUID
	TrainerID *uuid.UUID
	Limit     int
	Cursor    string
}

// ListResult is a page of classes plus the cursor for the next page, if any.
type ListResult struct {
	Items      []*Class
	NextCursor string
}
