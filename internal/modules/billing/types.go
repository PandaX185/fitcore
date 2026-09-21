// Package billing implements invoicing and payments for FitCore.
package billing

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("invoice not found")

// InvoiceStatus describes the lifecycle state of an invoice.
type InvoiceStatus string

const (
	StatusPending InvoiceStatus = "pending"
	StatusPaid    InvoiceStatus = "paid"
	StatusFailed  InvoiceStatus = "failed"
	StatusVoid    InvoiceStatus = "void"
)

// Invoice is the application-facing invoice record.
type Invoice struct {
	ID           uuid.UUID
	MemberID     uuid.UUID
	MembershipID uuid.UUID
	AmountCents  int64
	Currency     string
	Status       InvoiceStatus
	DueAt        time.Time
	PaidAt       *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
