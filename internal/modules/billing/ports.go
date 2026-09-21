package billing

import (
	"context"

	"github.com/google/uuid"
)

// InvoiceRepository is the persistence port for invoices.
type InvoiceRepository interface {
	Create(ctx context.Context, invoice *Invoice) error
	GetByID(ctx context.Context, id uuid.UUID) (*Invoice, error)
}
