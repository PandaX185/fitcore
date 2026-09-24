package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/PandaX185/fitcore/internal/modules/memberships"
)

// membershipRow is the GORM view of the memberships table.
type membershipRow struct {
	ID        uuid.UUID `gorm:"column:id"`
	MemberID  uuid.UUID `gorm:"column:member_id"`
	PackageID uuid.UUID `gorm:"column:package_id"`
	BranchID  uuid.UUID `gorm:"column:branch_id"`
	Status    string    `gorm:"column:status"`
	StartsOn  time.Time `gorm:"column:starts_on"`
	ExpiresOn time.Time `gorm:"column:expires_on"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (membershipRow) TableName() string { return "memberships" }

// MembershipRepository persists memberships via GORM.
type MembershipRepository struct {
	db *DB
}

func NewMembershipRepository(db *DB) *MembershipRepository {
	return &MembershipRepository{db: db}
}

func (r *MembershipRepository) Create(ctx context.Context, m *memberships.Membership) error {
	err := r.db.Gorm().WithContext(ctx).Create(toMembershipRow(m)).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return memberships.ErrDuplicateActive
	}
	return err
}

func (r *MembershipRepository) GetByID(ctx context.Context, id uuid.UUID) (*memberships.Membership, error) {
	var row membershipRow
	err := r.db.Gorm().WithContext(ctx).First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, memberships.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toMembership(), nil
}

func (r *MembershipRepository) ListByMember(ctx context.Context, memberID uuid.UUID) ([]*memberships.Membership, error) {
	var rows []membershipRow
	err := r.db.Gorm().WithContext(ctx).
		Where("member_id = ?", memberID).
		Order("starts_on DESC, id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*memberships.Membership, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toMembership())
	}
	return out, nil
}

func (r *MembershipRepository) Update(ctx context.Context, id uuid.UUID, patch *memberships.Patch) error {
	sets := map[string]any{"updated_at": time.Now().UTC()}
	if patch.Status != nil {
		sets["status"] = string(*patch.Status)
	}
	if patch.ExpiresAt != nil {
		sets["expires_on"] = *patch.ExpiresAt
	}

	res := r.db.Gorm().WithContext(ctx).Model(&membershipRow{}).Where("id = ?", id).Updates(sets)
	if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
		return memberships.ErrDuplicateActive
	}
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return memberships.ErrNotFound
	}
	return nil
}

func (r *MembershipRepository) HasActiveByMember(ctx context.Context, memberID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.Gorm().WithContext(ctx).
		Model(&membershipRow{}).
		Where("member_id = ? AND status = ?", memberID, memberships.StatusActive).
		Count(&count).Error
	return count > 0, err
}

func (r *MembershipRepository) FindActiveByMemberAndBranch(ctx context.Context, memberID, branchID uuid.UUID) (*memberships.Membership, error) {
	var row membershipRow
	err := r.db.Gorm().WithContext(ctx).
		Where("member_id = ? AND branch_id = ? AND status = ?", memberID, branchID, memberships.StatusActive).
		Order("starts_on DESC, id").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, memberships.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toMembership(), nil
}

func toMembershipRow(m *memberships.Membership) *membershipRow {
	return &membershipRow{
		ID:        m.ID,
		MemberID:  m.MemberID,
		PackageID: m.PackageID,
		BranchID:  m.BranchID,
		Status:    string(m.Status),
		StartsOn:  m.StartsAt,
		ExpiresOn: m.ExpiresAt,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

func (r membershipRow) toMembership() *memberships.Membership {
	return &memberships.Membership{
		ID:        r.ID,
		MemberID:  r.MemberID,
		PackageID: r.PackageID,
		BranchID:  r.BranchID,
		StartsAt:  r.StartsOn,
		ExpiresAt: r.ExpiresOn,
		Status:    memberships.Status(r.Status),
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}
