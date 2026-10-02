package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/PandaX185/fitcore/internal/modules/bookings"
)

// classBookingRow is the GORM view of the class_bookings table.
type classBookingRow struct {
	ID          uuid.UUID  `gorm:"column:id"`
	ClassID     uuid.UUID  `gorm:"column:class_id"`
	MemberID    uuid.UUID  `gorm:"column:member_id"`
	Status      string     `gorm:"column:status"`
	BookedAt    time.Time  `gorm:"column:created_at"`
	CancelledAt *time.Time `gorm:"column:cancelled_at"`
}

func (classBookingRow) TableName() string { return "class_bookings" }

// BookingRepository persists class bookings via GORM.
type BookingRepository struct {
	db *DB
}

func NewBookingRepository(db *DB) *BookingRepository {
	return &BookingRepository{db: db}
}

func (r *BookingRepository) Create(ctx context.Context, b *bookings.Booking) error {
	err := FromContext(ctx, r.db.Gorm()).WithContext(ctx).Create(toClassBookingRow(b)).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return bookings.ErrDuplicate
	}
	return err
}

func (r *BookingRepository) GetByID(ctx context.Context, id uuid.UUID) (*bookings.Booking, error) {
	var row classBookingRow
	err := FromContext(ctx, r.db.Gorm()).WithContext(ctx).First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, bookings.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toBooking(), nil
}

func (r *BookingRepository) Cancel(ctx context.Context, b *bookings.Booking) error {
	res := FromContext(ctx, r.db.Gorm()).WithContext(ctx).
		Model(&classBookingRow{}).
		Where("id = ? AND status = ?", b.ID, bookings.StatusBooked).
		Updates(map[string]any{
			"status":       string(b.Status),
			"cancelled_at": b.CancelledAt,
		})
	if res.Error != nil {
		return res.Error
	}
	// Zero rows means the booking was already cancelled (or never existed);
	// the service re-reads to tell the two apart.
	if res.RowsAffected == 0 {
		return bookings.ErrNotFound
	}
	return nil
}

// ListByClass returns one class's bookings ordered by (created_at, id),
// applying the exclusive cursor key and capping the result at the requested
// limit.
func (r *BookingRepository) ListByClass(ctx context.Context, q *bookings.ClassListQuery) ([]*bookings.Booking, error) {
	db := FromContext(ctx, r.db.Gorm()).WithContext(ctx).Where("class_id = ?", q.ClassID)
	if q.AfterID != uuid.Nil {
		db = db.Where("(created_at, id) > (?::timestamptz, ?)", q.AfterBookedAt, q.AfterID)
	}
	var rows []classBookingRow
	err := db.Order("created_at, id").Limit(q.Limit).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*bookings.Booking, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toBooking())
	}
	return out, nil
}

func (r *BookingRepository) CountActiveByClass(ctx context.Context, classID uuid.UUID) (int, error) {
	var count int64
	err := FromContext(ctx, r.db.Gorm()).WithContext(ctx).
		Model(&classBookingRow{}).
		Where("class_id = ? AND status = ?", classID, bookings.StatusBooked).
		Count(&count).Error
	return int(count), err
}

func toClassBookingRow(b *bookings.Booking) *classBookingRow {
	return &classBookingRow{
		ID:          b.ID,
		ClassID:     b.ClassID,
		MemberID:    b.MemberID,
		Status:      string(b.Status),
		BookedAt:    b.BookedAt,
		CancelledAt: b.CancelledAt,
	}
}

func (r classBookingRow) toBooking() *bookings.Booking {
	return &bookings.Booking{
		ID:          r.ID,
		ClassID:     r.ClassID,
		MemberID:    r.MemberID,
		Status:      bookings.BookingStatus(r.Status),
		BookedAt:    r.BookedAt,
		CancelledAt: r.CancelledAt,
		CreatedAt:   r.BookedAt,
	}
}
