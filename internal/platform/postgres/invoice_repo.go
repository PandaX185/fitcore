package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/PandaX185/fitcore/internal/modules/billing"
)

// invoiceRow is the GORM view of the invoices table. issued_on is the domain's
// created/issued timestamp; paid_on maps to PaidAt.
type invoiceRow struct {
	ID           uuid.UUID  `gorm:"column:id"`
	MemberID     uuid.UUID  `gorm:"column:member_id"`
	MembershipID uuid.UUID  `gorm:"column:membership_id"`
	AmountCents  int64      `gorm:"column:amount_cents"`
	Currency     string     `gorm:"column:currency"`
	Status       string     `gorm:"column:status"`
	DueAt        time.Time  `gorm:"column:due_at"`
	PaidOn       *time.Time `gorm:"column:paid_on"`
	IssuedOn     time.Time  `gorm:"column:issued_on"`
	UpdatedAt    time.Time  `gorm:"column:updated_at"`
}

func (invoiceRow) TableName() string { return "invoices" }

// InvoiceRepository persists invoices via GORM.
type InvoiceRepository struct {
	db *DB
}

func NewInvoiceRepository(db *DB) *InvoiceRepository {
	return &InvoiceRepository{db: db}
}

func (r *InvoiceRepository) Create(ctx context.Context, inv *billing.Invoice) error {
	err := r.db.Gorm().WithContext(ctx).Create(toInvoiceRow(inv)).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return billing.ErrInvalidInput
	}
	return err
}

func (r *InvoiceRepository) GetByID(ctx context.Context, id uuid.UUID) (*billing.Invoice, error) {
	var row invoiceRow
	err := r.db.Gorm().WithContext(ctx).First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, billing.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toInvoice(), nil
}

func (r *InvoiceRepository) ListByMember(ctx context.Context, memberID uuid.UUID) ([]*billing.Invoice, error) {
	var rows []invoiceRow
	err := r.db.Gorm().WithContext(ctx).
		Where("member_id = ?", memberID).
		Order("issued_on DESC, id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*billing.Invoice, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toInvoice())
	}
	return out, nil
}

// Update applies a partial patch. paid_on is derived from a transition to the
// paid status and cleared when leaving it.
func (r *InvoiceRepository) Update(ctx context.Context, id uuid.UUID, patch *billing.Patch) error {
	sets := map[string]any{"updated_at": time.Now().UTC()}
	if patch.Status != nil {
		sets["status"] = string(*patch.Status)
		now := time.Now().UTC()
		if *patch.Status == billing.StatusPaid {
			sets["paid_on"] = now
		} else {
			sets["paid_on"] = nil
		}
	}
	if patch.DueAt != nil {
		sets["due_at"] = *patch.DueAt
	}

	res := r.db.Gorm().WithContext(ctx).Model(&invoiceRow{}).Where("id = ?", id).Updates(sets)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return billing.ErrNotFound
	}
	return nil
}

func toInvoiceRow(inv *billing.Invoice) *invoiceRow {
	return &invoiceRow{
		ID:           inv.ID,
		MemberID:     inv.MemberID,
		MembershipID: inv.MembershipID,
		AmountCents:  inv.AmountCents,
		Currency:     inv.Currency,
		Status:       string(inv.Status),
		DueAt:        inv.DueAt,
		PaidOn:       inv.PaidAt,
		IssuedOn:     inv.CreatedAt,
		UpdatedAt:    inv.UpdatedAt,
	}
}

func (r invoiceRow) toInvoice() *billing.Invoice {
	return &billing.Invoice{
		ID:           r.ID,
		MemberID:     r.MemberID,
		MembershipID: r.MembershipID,
		AmountCents:  r.AmountCents,
		Currency:     r.Currency,
		Status:       billing.InvoiceStatus(r.Status),
		DueAt:        r.DueAt,
		PaidAt:       r.PaidOn,
		CreatedAt:    r.IssuedOn,
		UpdatedAt:    r.UpdatedAt,
	}
}
