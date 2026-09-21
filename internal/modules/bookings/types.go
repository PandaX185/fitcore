// Package bookings models members reserving seats in scheduled classes.
package bookings

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("booking not found")

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
