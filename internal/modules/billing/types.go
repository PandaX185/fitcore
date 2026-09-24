// Package billing models invoices issued to members for their memberships.
package billing

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound           = errors.New("invoice not found")
	ErrInvalidInput       = errors.New("invalid invoice input")
	ErrMemberNotFound     = errors.New("member not found")
	ErrMembershipNotFound = errors.New("membership not found")
)

// InvoiceStatus describes the lifecycle of an invoice.
type InvoiceStatus string

const (
	StatusPending InvoiceStatus = "pending"
	StatusPaid    InvoiceStatus = "paid"
	StatusFailed  InvoiceStatus = "failed"
	StatusVoid    InvoiceStatus = "void"
)

// Invoice is a charge against a member's membership.
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

// Patch is a partial update. Nil fields are left unchanged.
type Patch struct {
	Status *InvoiceStatus
	DueAt  *time.Time
}
