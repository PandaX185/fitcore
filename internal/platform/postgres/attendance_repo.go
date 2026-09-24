package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/PandaX185/fitcore/internal/modules/attendance"
)

// attendanceRow is the GORM view of the attendance table.
type attendanceRow struct {
	ID           uuid.UUID  `gorm:"column:id"`
	MemberID     uuid.UUID  `gorm:"column:member_id"`
	BranchID     uuid.UUID  `gorm:"column:branch_id"`
	MembershipID uuid.UUID  `gorm:"column:membership_id"`
	CheckedInAt  time.Time  `gorm:"column:checked_in_at"`
	CheckedOutAt *time.Time `gorm:"column:checked_out_at"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
}

func (attendanceRow) TableName() string { return "attendance" }

// AttendanceRepository persists attendance visits via GORM.
type AttendanceRepository struct {
	db *DB
}

func NewAttendanceRepository(db *DB) *AttendanceRepository {
	return &AttendanceRepository{db: db}
}

func (r *AttendanceRepository) Create(ctx context.Context, a *attendance.Attendance) error {
	return r.db.Gorm().WithContext(ctx).Create(toAttendanceRow(a)).Error
}

func (r *AttendanceRepository) GetByID(ctx context.Context, id uuid.UUID) (*attendance.Attendance, error) {
	var row attendanceRow
	err := r.db.Gorm().WithContext(ctx).First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, attendance.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toAttendance(), nil
}

func (r *AttendanceRepository) ListByMember(ctx context.Context, memberID uuid.UUID) ([]*attendance.Attendance, error) {
	var rows []attendanceRow
	err := r.db.Gorm().WithContext(ctx).
		Where("member_id = ?", memberID).
		Order("checked_in_at DESC, id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*attendance.Attendance, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toAttendance())
	}
	return out, nil
}

func (r *AttendanceRepository) FindOpenByMember(ctx context.Context, memberID uuid.UUID) (*attendance.Attendance, error) {
	var row attendanceRow
	err := r.db.Gorm().WithContext(ctx).
		Where("member_id = ? AND checked_out_at IS NULL", memberID).
		Order("checked_in_at DESC, id").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, attendance.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toAttendance(), nil
}

// Close stamps checked_out_at only when the record is still open; a zero-row
// update maps to ErrNotFound (including the already-closed race).
func (r *AttendanceRepository) Close(ctx context.Context, a *attendance.Attendance) error {
	res := r.db.Gorm().WithContext(ctx).
		Model(&attendanceRow{}).
		Where("id = ? AND checked_out_at IS NULL", a.ID).
		Update("checked_out_at", a.CheckedOutAt)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return attendance.ErrNotFound
	}
	return nil
}

func toAttendanceRow(a *attendance.Attendance) *attendanceRow {
	return &attendanceRow{
		ID:           a.ID,
		MemberID:     a.MemberID,
		BranchID:     a.BranchID,
		MembershipID: a.MembershipID,
		CheckedInAt:  a.CheckedInAt,
		CheckedOutAt: a.CheckedOutAt,
		CreatedAt:    a.CreatedAt,
	}
}

func (r attendanceRow) toAttendance() *attendance.Attendance {
	return &attendance.Attendance{
		ID:           r.ID,
		MemberID:     r.MemberID,
		BranchID:     r.BranchID,
		MembershipID: r.MembershipID,
		CheckedInAt:  r.CheckedInAt,
		CheckedOutAt: r.CheckedOutAt,
		CreatedAt:    r.CreatedAt,
	}
}
