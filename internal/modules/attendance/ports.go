package attendance

import (
	"context"

	"github.com/google/uuid"
)

// AttendanceRepository is the persistence port for attendance records.
type AttendanceRepository interface {
	Create(ctx context.Context, a *Attendance) error
	GetByID(ctx context.Context, id uuid.UUID) (*Attendance, error)
	GetOpen(ctx context.Context, memberID uuid.UUID) (*Attendance, error)
	Update(ctx context.Context, a *Attendance) error
}
