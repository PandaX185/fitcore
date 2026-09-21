// Package attendance models member check-ins.
package attendance

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound           = errors.New("attendance record not found")
	ErrNoActiveMembership = errors.New("member has no active membership")
)

// Attendance records a single in/out visit of a member at a branch.
type Attendance struct {
	ID           uuid.UUID
	MemberID     uuid.UUID
	BranchID     uuid.UUID
	MembershipID *uuid.UUID
	CheckedInAt  time.Time
	CheckedOutAt *time.Time
	CreatedAt    time.Time
}
