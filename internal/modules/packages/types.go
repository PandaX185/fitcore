// Package packages models purchasable membership packages.
package packages

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound      = errors.New("package not found")
	ErrInvalid       = errors.New("invalid package")
	ErrDuplicateName = errors.New("package name already in use")
)

// Package is a purchasable membership product.
type Package struct {
	ID           uuid.UUID
	Name         string
	DurationDays int
	PriceCents   int64
	Currency     string
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Patch is a partial update. Nil fields are left unchanged.
type Patch struct {
	Name         *string
	DurationDays *int
	PriceCents   *int64
	Currency     *string
	Active       *bool
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

// ListResult is a page of packages plus the cursor for the next page, if any.
type ListResult struct {
	Items      []*Package
	NextCursor string
}
