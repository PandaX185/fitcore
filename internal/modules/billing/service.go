package billing

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/members"
	"github.com/PandaX185/fitcore/internal/modules/memberships"
)

// Service implements the invoice business rules on the ports.
type Service struct {
	repo        InvoiceRepository
	members     MemberReader
	memberships MembershipReader
}

func NewService(repo InvoiceRepository, members MemberReader, memberships MembershipReader) *Service {
	return &Service{repo: repo, members: members, memberships: memberships}
}

// Get returns the invoice with the given ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Invoice, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	return s.repo.GetByID(ctx, id)
}

// Create issues an invoice for a membership. The member and membership must
// exist; the invoice starts pending.
func (s *Service) Create(ctx context.Context, memberID, membershipID uuid.UUID, amountCents int64, currency string, dueAt time.Time) (*Invoice, error) {
	if memberID == uuid.Nil || membershipID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	if amountCents < 0 || dueAt.IsZero() {
		return nil, ErrInvalidInput
	}
	currency, err := normalizeCurrency(currency)
	if err != nil {
		return nil, ErrInvalidInput
	}
	if _, err := s.members.Get(ctx, memberID); err != nil {
		if errors.Is(err, members.ErrNotFound) {
			return nil, ErrMemberNotFound
		}
		return nil, err
	}
	if _, err := s.memberships.Get(ctx, membershipID); err != nil {
		if errors.Is(err, memberships.ErrNotFound) {
			return nil, ErrMembershipNotFound
		}
		return nil, err
	}
	now := time.Now().UTC()
	inv := &Invoice{
		ID:           uuid.New(),
		MemberID:     memberID,
		MembershipID: membershipID,
		AmountCents:  amountCents,
		Currency:     currency,
		Status:       StatusPending,
		DueAt:        dueAt,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.Create(ctx, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

// Update applies a partial patch. Moving an invoice to paid stamps paid_at
// automatically; moving it out of paid clears the stamp.
func (s *Service) Update(ctx context.Context, id uuid.UUID, patch Patch) (*Invoice, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if patch.Status != nil {
		switch *patch.Status {
		case StatusPending, StatusPaid, StatusFailed, StatusVoid:
		default:
			return nil, ErrInvalidInput
		}
		if existing.Status == StatusPaid && *patch.Status != StatusPaid {
			return nil, ErrInvalidInput
		}
	}
	if patch.DueAt != nil && patch.DueAt.IsZero() {
		return nil, ErrInvalidInput
	}
	if err := s.repo.Update(ctx, id, &patch); err != nil {
		return nil, err
	}
	updated, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// ListByMember returns a member's invoices, newest first.
func (s *Service) ListByMember(ctx context.Context, memberID uuid.UUID) ([]*Invoice, error) {
	if memberID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	return s.repo.ListByMember(ctx, memberID)
}

// normalizeCurrency uppercases and validates a 3-letter currency code.
func normalizeCurrency(code string) (string, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	if len(code) != 3 {
		return "", ErrInvalidInput
	}
	for _, r := range code {
		if r < 'A' || r > 'Z' {
			return "", ErrInvalidInput
		}
	}
	return code, nil
}
