package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/PandaX185/fitcore/internal/modules/classes"
)

// classRow is the GORM view of the classes table.
type classRow struct {
	ID        uuid.UUID  `gorm:"column:id"`
	BranchID  uuid.UUID  `gorm:"column:branch_id"`
	TrainerID *uuid.UUID `gorm:"column:trainer_id"`
	Name      string     `gorm:"column:name"`
	Capacity  int        `gorm:"column:capacity"`
	StartsAt  time.Time  `gorm:"column:starts_at"`
	EndsAt    time.Time  `gorm:"column:ends_at"`
	CreatedAt time.Time  `gorm:"column:created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at"`
}

func (classRow) TableName() string { return "classes" }

// ClassRepository persists classes via GORM.
type ClassRepository struct {
	db *DB
}

func NewClassRepository(db *DB) *ClassRepository {
	return &ClassRepository{db: db}
}

func (r *ClassRepository) Create(ctx context.Context, c *classes.Class) error {
	return r.db.Gorm().WithContext(ctx).Create(toClassRow(c)).Error
}

func (r *ClassRepository) GetByID(ctx context.Context, id uuid.UUID) (*classes.Class, error) {
	var row classRow
	err := r.db.Gorm().WithContext(ctx).First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, classes.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toClass(), nil
}

func (r *ClassRepository) Update(ctx context.Context, id uuid.UUID, patch *classes.Patch) error {
	sets := map[string]any{"updated_at": time.Now().UTC()}
	if patch.TrainerID != nil {
		sets["trainer_id"] = *patch.TrainerID
	}
	if patch.Name != nil {
		sets["name"] = *patch.Name
	}
	if patch.StartsAt != nil {
		sets["starts_at"] = *patch.StartsAt
	}
	if patch.EndsAt != nil {
		sets["ends_at"] = *patch.EndsAt
	}
	if patch.Capacity != nil {
		sets["capacity"] = *patch.Capacity
	}

	res := r.db.Gorm().WithContext(ctx).Model(&classRow{}).Where("id = ?", id).Updates(sets)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return classes.ErrNotFound
	}
	return nil
}

func (r *ClassRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.Gorm().WithContext(ctx).Where("id = ?", id).Delete(&classRow{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return classes.ErrNotFound
	}
	return nil
}

func (r *ClassRepository) List(ctx context.Context, branchID *uuid.UUID, trainerID *uuid.UUID) ([]*classes.Class, error) {
	q := r.db.Gorm().WithContext(ctx)
	if branchID != nil && *branchID != uuid.Nil {
		q = q.Where("branch_id = ?", *branchID)
	}
	if trainerID != nil && *trainerID != uuid.Nil {
		q = q.Where("trainer_id = ?", *trainerID)
	}
	var rows []classRow
	err := q.Order("starts_at, id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*classes.Class, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toClass())
	}
	return out, nil
}

func toClassRow(c *classes.Class) *classRow {
	var trainerID *uuid.UUID
	if c.TrainerID != nil && *c.TrainerID != uuid.Nil {
		tr := *c.TrainerID
		trainerID = &tr
	}
	return &classRow{
		ID:        c.ID,
		BranchID:  c.BranchID,
		TrainerID: trainerID,
		Name:      c.Name,
		Capacity:  c.Capacity,
		StartsAt:  c.StartsAt,
		EndsAt:    c.EndsAt,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func (r classRow) toClass() *classes.Class {
	return &classes.Class{
		ID:        r.ID,
		BranchID:  r.BranchID,
		TrainerID: r.TrainerID,
		Name:      r.Name,
		Capacity:  r.Capacity,
		StartsAt:  r.StartsAt,
		EndsAt:    r.EndsAt,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}
