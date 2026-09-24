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
