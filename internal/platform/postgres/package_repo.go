package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/PandaX185/fitcore/internal/modules/packages"
)

// membershipPackageRow is the GORM view of the membership_packages table.
type membershipPackageRow struct {
	ID           uuid.UUID `gorm:"column:id"`
	Name         string    `gorm:"column:name"`
	DurationDays int       `gorm:"column:duration_days"`
	PriceCents   int64     `gorm:"column:price_cents"`
	Currency     string    `gorm:"column:currency"`
	Active       bool      `gorm:"column:active"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (membershipPackageRow) TableName() string { return "membership_packages" }

// PackageRepository persists membership packages via GORM.
type PackageRepository struct {
	db *DB
}

func NewPackageRepository(db *DB) *PackageRepository {
	return &PackageRepository{db: db}
}

func (r *PackageRepository) Create(ctx context.Context, p *packages.Package) error {
	err := r.db.Gorm().WithContext(ctx).Create(toMembershipPackageRow(p)).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return packages.ErrDuplicateName
	}
	return err
}

func (r *PackageRepository) GetByID(ctx context.Context, id uuid.UUID) (*packages.Package, error) {
	var row membershipPackageRow
	err := r.db.Gorm().WithContext(ctx).First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, packages.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toPackage(), nil
}

func (r *PackageRepository) List(ctx context.Context) ([]*packages.Package, error) {
	var rows []membershipPackageRow
	err := r.db.Gorm().WithContext(ctx).Order("name, id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*packages.Package, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toPackage())
	}
	return out, nil
}

// Update applies a partial patch. It reports ErrNotFound when no row matches
// and ErrDuplicateName on a name collision, and always refreshes updated_at.
func (r *PackageRepository) Update(ctx context.Context, id uuid.UUID, patch *packages.Patch) error {
	sets := map[string]any{"updated_at": time.Now().UTC()}
	if patch.Name != nil {
		sets["name"] = *patch.Name
	}
	if patch.DurationDays != nil {
		sets["duration_days"] = *patch.DurationDays
	}
	if patch.PriceCents != nil {
		sets["price_cents"] = *patch.PriceCents
	}
	if patch.Currency != nil {
		sets["currency"] = *patch.Currency
	}
	if patch.Active != nil {
		sets["active"] = *patch.Active
	}

	res := r.db.Gorm().WithContext(ctx).Model(&membershipPackageRow{}).Where("id = ?", id).Updates(sets)
	if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
		return packages.ErrDuplicateName
	}
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return packages.ErrNotFound
	}
	return nil
}

func toMembershipPackageRow(p *packages.Package) *membershipPackageRow {
	return &membershipPackageRow{
		ID:           p.ID,
		Name:         p.Name,
		DurationDays: p.DurationDays,
		PriceCents:   p.PriceCents,
		Currency:     p.Currency,
		Active:       p.Active,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

func (r membershipPackageRow) toPackage() *packages.Package {
	return &packages.Package{
		ID:           r.ID,
		Name:         r.Name,
		DurationDays: r.DurationDays,
		PriceCents:   r.PriceCents,
		Currency:     r.Currency,
		Active:       r.Active,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}
