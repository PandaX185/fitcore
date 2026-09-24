package bookings

import (
	"context"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/classes"
	"github.com/PandaX185/fitcore/internal/modules/members"
)

// BookingRepository is the persistence port for class bookings.
type BookingRepository interface {
	Create(ctx context.Context, b *Booking) error
	GetByID(ctx context.Context, id uuid.UUID) (*Booking, error)
	Cancel(ctx context.Context, b *Booking) error
	ListByClass(ctx context.Context, classID uuid.UUID) ([]*Booking, error)
	CountActiveByClass(ctx context.Context, classID uuid.UUID) (int, error)
}

// ClassReader is the slice of the classes store bookings needs for capacity
// and existence checks.
type ClassReader interface {
	Get(ctx context.Context, id uuid.UUID) (*classes.Class, error)
}

// MemberReader is the slice of the members store bookings needs to reject
// bookings for unknown members.
type MemberReader interface {
	Get(ctx context.Context, id uuid.UUID) (*members.Member, error)
}
