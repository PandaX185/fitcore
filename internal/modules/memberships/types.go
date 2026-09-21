// Package memberships implements membership purchasing and lifecycle use
// cases for FitCore.
package memberships

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound     = errors.New("membership or package not found")
	ErrInvalidInput = errors.New("invalid membership input")
)

// Status describes the lifecycle state of a membership.
type Status string

const (
	StatusActive  Status = "active"
	StatusFrozen  Status = "frozen"
	StatusExpired Status = "expired"
)

// Membership is the application-facing membership record.
type Membership struct {
	ID        uuid.UUID
	MemberID  uuid.UUID
	PackageID uuid.UUID
	BranchID  uuid.UUID
	StartsAt  time.Time
	ExpiresAt time.Time
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
}

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
