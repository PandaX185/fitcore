package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/PandaX185/fitcore/internal/modules/trainers"
)

// trainerRow is the GORM view of the trainers table.
type trainerRow struct {
	ID        uuid.UUID `gorm:"column:id"`
	BranchID  uuid.UUID `gorm:"column:branch_id"`
	Name      string    `gorm:"column:name"`
	Email     string    `gorm:"column:email"`
	Phone     string    `gorm:"column:phone"`
	Active    bool      `gorm:"column:active"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (trainerRow) TableName() string { return "trainers" }

// TrainerRepository persists trainers via GORM.
type TrainerRepository struct {
	db *DB
}

func NewTrainerRepository(db *DB) *TrainerRepository {
	return &TrainerRepository{db: db}
}

func (r *TrainerRepository) Create(ctx context.Context, t *trainers.Trainer) error {
	err := r.db.Gorm().WithContext(ctx).Create(toTrainerRow(t)).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return trainers.ErrDuplicateEmail
	}
	return err
}

func (r *TrainerRepository) GetByID(ctx context.Context, id uuid.UUID) (*trainers.Trainer, error) {
	var row trainerRow
	err := r.db.Gorm().WithContext(ctx).First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, trainers.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toTrainer(), nil
}

func (r *TrainerRepository) ListByBranch(ctx context.Context, branchID uuid.UUID) ([]*trainers.Trainer, error) {
	var rows []trainerRow
	err := r.db.Gorm().WithContext(ctx).
		Where("branch_id = ?", branchID).
		Order("name, id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*trainers.Trainer, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toTrainer())
	}
	return out, nil
}

func (r *TrainerRepository) Update(ctx context.Context, id uuid.UUID, patch *trainers.Patch) error {
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
	if patch.Active != nil {
		sets["active"] = *patch.Active
	}

	res := r.db.Gorm().WithContext(ctx).Model(&trainerRow{}).Where("id = ?", id).Updates(sets)
	if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
		return trainers.ErrDuplicateEmail
	}
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return trainers.ErrNotFound
	}
	return nil
}

func toTrainerRow(t *trainers.Trainer) *trainerRow {
	return &trainerRow{
		ID:        t.ID,
		BranchID:  t.BranchID,
		Name:      t.Name,
		Email:     t.Email,
		Phone:     t.Phone,
		Active:    t.Active,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

func (r trainerRow) toTrainer() *trainers.Trainer {
	return &trainers.Trainer{
		ID:        r.ID,
		BranchID:  r.BranchID,
		Name:      r.Name,
		Email:     r.Email,
		Phone:     r.Phone,
		Active:    r.Active,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}
