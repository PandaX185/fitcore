package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/PandaX185/fitcore/internal/modules/staff"
)

// staffRow is the GORM view of the staff table.
type staffRow struct {
	ID          uuid.UUID      `gorm:"column:id"`
	BranchID    uuid.UUID      `gorm:"column:branch_id"`
	Name        string         `gorm:"column:name"`
	Email       string         `gorm:"column:email"`
	Phone       string         `gorm:"column:phone"`
	Permissions permissionList `gorm:"column:permissions"`
	Active      bool           `gorm:"column:active"`
	CreatedAt   time.Time      `gorm:"column:created_at"`
	UpdatedAt   time.Time      `gorm:"column:updated_at"`
}

func (staffRow) TableName() string { return "staff" }

// StaffRepository persists staff records via GORM. It shares the staff table
// with authentication; password provisioning is out-of-band.
type StaffRepository struct {
	db *DB
}

func NewStaffRepository(db *DB) *StaffRepository {
	return &StaffRepository{db: db}
}

func (r *StaffRepository) Create(ctx context.Context, s *staff.Staff) error {
	err := r.db.Gorm().WithContext(ctx).Create(toStaffRow(s)).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return staff.ErrDuplicateEmail
	}
	return err
}

func (r *StaffRepository) GetByID(ctx context.Context, id uuid.UUID) (*staff.Staff, error) {
	var row staffRow
	err := r.db.Gorm().WithContext(ctx).First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, staff.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toStaff(), nil
}

func (r *StaffRepository) ListByBranch(ctx context.Context, branchID uuid.UUID) ([]*staff.Staff, error) {
	var rows []staffRow
	err := r.db.Gorm().WithContext(ctx).
		Where("branch_id = ?", branchID).
		Order("name, id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*staff.Staff, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toStaff())
	}
	return out, nil
}

func (r *StaffRepository) Update(ctx context.Context, id uuid.UUID, patch *staff.Patch) error {
	sets := map[string]any{"updated_at": time.Now().UTC()}
	if patch.Name != nil {
		sets["name"] = *patch.Name
	}
	if patch.Email != nil {
		sets["email"] = *patch.Email
	}
	if patch.Phone != nil {
		sets["phone"] = *patch.Phone
	}
	if patch.Permissions != nil {
		sets["permissions"] = permissionList(*patch.Permissions)
	}
	if patch.Active != nil {
		sets["active"] = *patch.Active
	}

	res := r.db.Gorm().WithContext(ctx).Model(&staffRow{}).Where("id = ?", id).Updates(sets)
	if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
		return staff.ErrDuplicateEmail
	}
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return staff.ErrNotFound
	}
	return nil
}

func toStaffRow(s *staff.Staff) *staffRow {
	return &staffRow{
		ID:          s.ID,
		BranchID:    s.BranchID,
		Name:        s.Name,
		Email:       s.Email,
		Phone:       s.Phone,
		Permissions: permissionList(s.Permissions),
		Active:      s.Active,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

func (r staffRow) toStaff() *staff.Staff {
	return &staff.Staff{
		ID:          r.ID,
		BranchID:    r.BranchID,
		Name:        r.Name,
		Email:       r.Email,
		Phone:       r.Phone,
		Permissions: r.Permissions,
		Active:      r.Active,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}
