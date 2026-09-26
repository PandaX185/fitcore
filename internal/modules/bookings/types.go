// Package bookings models members reserving seats in scheduled classes.
package bookings

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound       = errors.New("booking not found")
	ErrInvalidInput   = errors.New("invalid booking input")
	ErrClassNotFound  = errors.New("class not found")
	ErrMemberNotFound = errors.New("member not found")
	ErrClassFull      = errors.New("class is full")
	ErrDuplicate      = errors.New("member already booked this class")
)

// BookingStatus describes the lifecycle state of a class booking.
type BookingStatus string

const (
	StatusBooked    BookingStatus = "booked"
	StatusCancelled BookingStatus = "cancelled"
)

// Booking records a member's reservation in a class.
type Booking struct {
	ID          uuid.UUID
	ClassID     uuid.UUID
	MemberID    uuid.UUID
	Status      BookingStatus
	BookedAt    time.Time
	CancelledAt *time.Time
	CreatedAt   time.Time
}

// ClassListQuery is the paginated, class-scoped query handed to persistence.
type ClassListQuery struct {
	// ClassID scopes the result to one class.
	ClassID uuid.UUID
	// Limit is the maximum number of rows to return.
	Limit int
	// AfterBookedAt and AfterID form the exclusive cursor key into the
	// (created_at, id) ordering; AfterID is zero on the first page.
	// AfterBookedAt carries an RFC3339 timestamp as text; the domain
	// exposes created_at as Booking.BookedAt.
	AfterBookedAt string
	AfterID       uuid.UUID
}

// ClassListParams is the service-facing page request.
type ClassListParams struct {
	Limit  int
	Cursor string
}

// ClassListResult is a page of bookings plus the cursor for the next page,
// if any.
type ClassListResult struct {
	Items      []*Booking
	NextCursor string
}
