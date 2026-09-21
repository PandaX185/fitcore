// Package branches models FitCore's physical gym locations.
package branches

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("branch not found")
	ErrInvalid  = errors.New("invalid branch")
)

// Branch is a physical gym location.
type Branch struct {
	ID        uuid.UUID
	Name      string
	Address   string
	Latitude  float64
	Longitude float64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Patch is a partial update. Nil fields are left unchanged.
type Patch struct {
	Name      *string
	Address   *string
	Latitude  *float64
	Longitude *float64
}

// ListQuery is the searchable, paginated query handed to persistence.
type ListQuery struct {
	// Query matches name or address as a case-insensitive substring.
	Query string
	// Limit is the maximum number of rows to return.
	Limit int
	// AfterName and AfterID form the exclusive cursor key into the
	// (name, id) ordering; both zero-valued on the first page.
	AfterName string
	AfterID   uuid.UUID
}

// ListParams is the service-facing page request.
type ListParams struct {
	Query  string
	Limit  int
	Cursor string
}

// ListResult is a page of branches plus the cursor for the next page, if any.
type ListResult struct {
	Items      []*Branch
	NextCursor string
}
