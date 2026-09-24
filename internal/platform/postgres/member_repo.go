package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/PandaX185/fitcore/internal/modules/members"
)

// MemberRepository persists members via GORM.
type MemberRepository struct {
	db *DB
}

func NewMemberRepository(db *DB) *MemberRepository {
	return &MemberRepository{db: db}
}

func (r *MemberRepository) Create(ctx context.Context, m *members.Member) error {
	err := r.db.Gorm().WithContext(ctx).Create(m).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return members.ErrDuplicateEmail
	}
	return err
}

func (r *MemberRepository) GetByID(ctx context.Context, id uuid.UUID) (*members.Member, error) {
	var m members.Member
	err := r.db.Gorm().WithContext(ctx).First(&m, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, members.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// Update applies a partial patch. It reports ErrNotFound when no row matches,
// ErrDuplicateEmail when the email collides, and always refreshes updated_at.
func (r *MemberRepository) Update(ctx context.Context, id uuid.UUID, patch *members.Patch) error {
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
	if patch.Status != nil {
		sets["status"] = *patch.Status
	}

	res := r.db.Gorm().WithContext(ctx).Model(&members.Member{}).Where("id = ?", id).Updates(sets)
	if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
		return members.ErrDuplicateEmail
	}
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return members.ErrNotFound
	}
	return nil
}

// Delete removes a member by ID, reporting ErrNotFound when no row matches.
func (r *MemberRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.Gorm().WithContext(ctx).Where("id = ?", id).Delete(&members.Member{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return members.ErrNotFound
	}
	return nil
}

// List returns all members ordered by (name, id) for stable paging.
func (r *MemberRepository) List(ctx context.Context) ([]*members.Member, error) {
	var ms []*members.Member
	err := r.db.Gorm().WithContext(ctx).Order("name, id").Find(&ms).Error
	return ms, err
}
