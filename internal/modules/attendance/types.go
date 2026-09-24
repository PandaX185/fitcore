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
