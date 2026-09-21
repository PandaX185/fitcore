package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/PandaX185/fitcore/internal/modules/branches"
)

// BranchRepository persists branches via GORM.
type BranchRepository struct {
	db *DB
}

func NewBranchRepository(db *DB) *BranchRepository {
	return &BranchRepository{db: db}
}

func (r *BranchRepository) Create(ctx context.Context, b *branches.Branch) error {
	return r.db.Gorm().WithContext(ctx).Create(b).Error
}

func (r *BranchRepository) GetByID(ctx context.Context, id uuid.UUID) (*branches.Branch, error) {
	var b branches.Branch
	err := r.db.Gorm().WithContext(ctx).First(&b, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, branches.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// List returns rows ordered by (name, id), applying the search filter and the
// exclusive cursor key and capping the result at the requested limit.
func (r *BranchRepository) List(ctx context.Context, q *branches.ListQuery) ([]*branches.Branch, error) {
	db := r.db.Gorm().WithContext(ctx)
	if q.Query != "" {
		pattern := likePattern(q.Query)
		db = db.Where("(name ILIKE ? OR address ILIKE ?)", pattern, pattern)
	}
	if q.AfterID != uuid.Nil {
		db = db.Where("(name, id) > (?, ?)", q.AfterName, q.AfterID)
	}
	var bs []*branches.Branch
	err := db.Order("name, id").Limit(q.Limit).Find(&bs).Error
	return bs, err
}

// Update applies a partial patch. It reports ErrNotFound when no row matches,
// and always refreshes updated_at.
func (r *BranchRepository) Update(ctx context.Context, id uuid.UUID, patch *branches.Patch) error {
	sets := map[string]any{"updated_at": time.Now().UTC()}
	if patch.Name != nil {
		sets["name"] = *patch.Name
	}
	if patch.Address != nil {
		sets["address"] = *patch.Address
	}
	if patch.Latitude != nil {
		sets["latitude"] = *patch.Latitude
		sets["longitude"] = *patch.Longitude
	}

	res := r.db.Gorm().WithContext(ctx).Model(&branches.Branch{}).Where("id = ?", id).Updates(sets)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return branches.ErrNotFound
	}
	return nil
}

// likePattern escapes ILIKE wildcards so user input matches literally while
// still being treated as a contains search.
func likePattern(q string) string {
	q = strings.ReplaceAll(q, `\`, `\\`)
	q = strings.ReplaceAll(q, `%`, `\%`)
	q = strings.ReplaceAll(q, `_`, `\_`)
	return "%" + q + "%"
}
