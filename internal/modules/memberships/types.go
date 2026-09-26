// Package memberships implements membership purchasing and lifecycle use
// cases for FitCore.
package memberships

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound        = errors.New("membership not found")
	ErrInvalidInput    = errors.New("invalid membership input")
	ErrDuplicateActive = errors.New("member already has an active membership")
	ErrMemberNotFound  = errors.New("member not found")
	ErrPackageNotFound = errors.New("package not found")
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

// Patch is a partial update. Nil fields are left unchanged.
type Patch struct {
	Status    *Status
	ExpiresAt *time.Time
}

// MemberListQuery is the paginated, member-scoped query handed to
// persistence.
type MemberListQuery struct {
	// MemberID scopes the result to one member.
	MemberID uuid.UUID
	// Limit is the maximum number of rows to return.
	Limit int
	// AfterStartsAt and AfterID form the exclusive cursor key into the
	// (starts_on DESC, id) ordering; AfterID is zero on the first page.
	// AfterStartsAt carries an RFC3339 timestamp as text.
	AfterStartsAt string
	AfterID       uuid.UUID
}

// MemberListParams is the service-facing page request.
type MemberListParams struct {
	Limit  int
	Cursor string
}

// MemberListResult is a page of memberships plus the cursor for the next
// page, if any.
type MemberListResult struct {
	Items      []*Membership
	NextCursor string
}
