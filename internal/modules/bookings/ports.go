package bookings

import (
	"context"

	"github.com/google/uuid"
)

// BookingRepository is the persistence port for class bookings.
type BookingRepository interface {
	Create(ctx context.Context, b *Booking) error
	GetByID(ctx context.Context, id uuid.UUID) (*Booking, error)
	Cancel(ctx context.Context, b *Booking) error
	CountActiveByClass(ctx context.Context, classID uuid.UUID) (int, error)
}
