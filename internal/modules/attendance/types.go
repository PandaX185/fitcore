// Package attendance models member visits to branches: check-in opens a visit,
// check-out closes it.
package attendance

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound           = errors.New("attendance record not found")
	ErrInvalidInput       = errors.New("invalid attendance input")
	ErrMemberNotFound     = errors.New("member not found")
	ErrNoActiveMembership = errors.New("member has no active membership")
	ErrAlreadyCheckedIn   = errors.New("member already checked in")
	ErrNoOpenRecord       = errors.New("member has no open attendance record")
	ErrAlreadyCheckedOut  = errors.New("attendance record already closed")
)

// Attendance is a single visit.
type Attendance struct {
	ID           uuid.UUID
	MemberID     uuid.UUID
	BranchID     uuid.UUID
	MembershipID uuid.UUID
	CheckedInAt  time.Time
	CheckedOutAt *time.Time
	CreatedAt    time.Time
}

// MemberListQuery is the paginated, member-scoped query handed to
// persistence.
type MemberListQuery struct {
	// MemberID scopes the result to one member.
	MemberID uuid.UUID
	// Limit is the maximum number of rows to return.
	Limit int
	// AfterCheckedInAt and AfterID form the exclusive cursor key into the
	// (checked_in_at DESC, id) ordering; AfterID is zero on the first page.
	// AfterCheckedInAt carries an RFC3339 timestamp as text.
	AfterCheckedInAt string
	AfterID          uuid.UUID
}

// MemberListParams is the service-facing page request.
type MemberListParams struct {
	Limit  int
	Cursor string
}

// MemberListResult is a page of attendance records plus the cursor for the
// next page, if any.
type MemberListResult struct {
	Items      []*Attendance
	NextCursor string
}
