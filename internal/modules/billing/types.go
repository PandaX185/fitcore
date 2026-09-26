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

// MemberListQuery is the paginated, member-scoped query handed to
// persistence.
type MemberListQuery struct {
	// MemberID scopes the result to one member.
	MemberID uuid.UUID
	// Limit is the maximum number of rows to return.
	Limit int
	// AfterIssuedAt and AfterID form the exclusive cursor key into the
	// (issued_on DESC, id) ordering; AfterID is zero on the first page.
	// AfterIssuedAt carries an RFC3339 timestamp as text; the domain
	// exposes issued_on as Invoice.CreatedAt.
	AfterIssuedAt string
	AfterID       uuid.UUID
}

// MemberListParams is the service-facing page request.
type MemberListParams struct {
	Limit  int
	Cursor string
}

// MemberListResult is a page of invoices plus the cursor for the next page,
// if any.
type MemberListResult struct {
	Items      []*Invoice
	NextCursor string
}
