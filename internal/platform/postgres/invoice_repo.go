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
	err := FromContext(ctx, r.db.Gorm()).WithContext(ctx).Create(toInvoiceRow(inv)).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return billing.ErrDuplicate
	}
	return err
}

func (r *InvoiceRepository) GetByID(ctx context.Context, id uuid.UUID) (*billing.Invoice, error) {
	var row invoiceRow
	err := FromContext(ctx, r.db.Gorm()).WithContext(ctx).First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, billing.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toInvoice(), nil
}

// ListByMember returns one member's invoices ordered by
// (issued_on DESC, id), applying the exclusive cursor key and capping the
// result at the requested limit.
func (r *InvoiceRepository) ListByMember(ctx context.Context, q *billing.MemberListQuery) ([]*billing.Invoice, error) {
	db := FromContext(ctx, r.db.Gorm()).WithContext(ctx).Where("member_id = ?", q.MemberID)
	if q.AfterID != uuid.Nil {
		db = db.Where("(issued_on < ?::timestamptz OR (issued_on = ?::timestamptz AND id > ?))",
			q.AfterIssuedAt, q.AfterIssuedAt, q.AfterID)
	}
	var rows []invoiceRow
	err := db.Order("issued_on DESC, id").Limit(q.Limit).Find(&rows).Error
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
// paid status and cleared when leaving it. The paid transition is a single
// conditional write (WHERE id AND status IN ('pending','failed')) so
// concurrent payers serialize: the loser updates zero rows and gets
// ErrNotFound, letting the service tell a missing invoice from a lost race.
// One timestamp backs both paid_on and updated_at.
func (r *InvoiceRepository) Update(ctx context.Context, id uuid.UUID, patch *billing.Patch) error {
	now := time.Now().UTC()
	sets := map[string]any{"updated_at": now}
	db := FromContext(ctx, r.db.Gorm()).WithContext(ctx).Model(&invoiceRow{})
	if patch.Status != nil {
		sets["status"] = string(*patch.Status)
		if *patch.Status == billing.StatusPaid {
			sets["paid_on"] = now
			db = db.Where("id = ? AND status IN ?", id, []string{string(billing.StatusPending), string(billing.StatusFailed)})
		} else {
			sets["paid_on"] = nil
			db = db.Where("id = ?", id)
		}
	} else {
		db = db.Where("id = ?", id)
	}
	if patch.DueAt != nil {
		sets["due_at"] = *patch.DueAt
	}

	res := db.Updates(sets)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return billing.ErrNotFound
	}
	return nil
}

// HasPendingByMember reports whether the member owes an invoice still in
// the pending status.
func (r *InvoiceRepository) HasPendingByMember(ctx context.Context, memberID uuid.UUID) (bool, error) {
	var count int64
	err := FromContext(ctx, r.db.Gorm()).WithContext(ctx).
		Model(&invoiceRow{}).
		Where("member_id = ? AND status = ?", memberID, string(billing.StatusPending)).
		Count(&count).Error
	return count > 0, err
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
