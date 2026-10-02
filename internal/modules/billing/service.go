package billing

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/members"
	"github.com/PandaX185/fitcore/internal/modules/memberships"
	"github.com/PandaX185/fitcore/internal/paging"
	"github.com/PandaX185/fitcore/internal/transact"
)

// Service implements the invoice business rules on the ports.
type Service struct {
	repo        InvoiceRepository
	members     MemberReader
	memberships MembershipReader
	tx          transact.Transactor
}

func NewService(repo InvoiceRepository, members MemberReader, memberships MembershipReader, tx transact.Transactor) *Service {
	return &Service{repo: repo, members: members, memberships: memberships, tx: tx}
}

// Get returns the invoice with the given ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Invoice, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	return s.repo.GetByID(ctx, id)
}

// Create issues an invoice for a membership. The member and membership must
// exist and the membership must belong to the member; the invoice starts
// pending. Duplicate invoice terms pass the store's ErrDuplicate through.
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
	mship, err := s.memberships.Get(ctx, membershipID)
	if err != nil {
		if errors.Is(err, memberships.ErrNotFound) {
			return nil, ErrMembershipNotFound
		}
		return nil, err
	}
	if mship.MemberID != memberID {
		return nil, ErrInvalidInput
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
	err = s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		return s.repo.Create(txCtx, inv)
	})
	if err != nil {
		return nil, err
	}
	return inv, nil
}

// Update applies a partial patch. Moving an invoice to paid stamps paid_at
// automatically; moving it out of paid clears the stamp.
//
// Terminal states: void is terminal — any status change out of void is
// ErrStateConflict (409). Paid is append-only in the same spirit: leaving
// paid is ErrInvalidInput. The paid transition itself is a single
// conditional write (pending/failed only); when it updates zero rows the
// store reports ErrNotFound and the service re-reads to tell a missing
// invoice (ErrNotFound) from a lost race (ErrStateConflict).
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
		if existing.Status == StatusVoid && *patch.Status != StatusVoid {
			return nil, ErrStateConflict
		}
		if existing.Status == StatusPaid && *patch.Status != StatusPaid {
			return nil, ErrInvalidInput
		}
	}
	if patch.DueAt != nil && patch.DueAt.IsZero() {
		return nil, ErrInvalidInput
	}
	toPaid := patch.Status != nil && *patch.Status == StatusPaid
	var updated *Invoice
	err = s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repo.Update(txCtx, id, &patch); err != nil {
			if errors.Is(err, ErrNotFound) && toPaid {
				if _, getErr := s.repo.GetByID(txCtx, id); errors.Is(getErr, ErrNotFound) {
					return ErrNotFound
				} else if getErr != nil {
					return getErr
				}
				return ErrStateConflict
			}
			return err
		}
		u, err := s.repo.GetByID(txCtx, id)
		if err != nil {
			return err
		}
		updated = u
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// ListByMember returns a member's invoices, newest first.
// ListByMember returns one page of a member's invoices ordered by
// (issued_on DESC, id), with an opaque cursor for the next page when more
// rows remain.
func (s *Service) ListByMember(ctx context.Context, memberID uuid.UUID, p MemberListParams) (*MemberListResult, error) {
	if memberID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	limit := paging.Limit(p.Limit)
	cursor, err := paging.DecodeCursor(p.Cursor)
	if err != nil {
		return nil, ErrInvalidInput
	}
	if cursor.Key != "" {
		if _, err := time.Parse(time.RFC3339Nano, cursor.Key); err != nil {
			return nil, ErrInvalidInput
		}
	}
	items, err := s.repo.ListByMember(ctx, &MemberListQuery{
		MemberID:      memberID,
		Limit:         limit + 1,
		AfterIssuedAt: cursor.Key,
		AfterID:       cursor.ID,
	})
	if err != nil {
		return nil, err
	}
	res := &MemberListResult{Items: items}
	if len(items) > limit {
		res.Items = items[:limit]
		last := items[limit-1]
		res.NextCursor = paging.Cursor{Key: last.CreatedAt.UTC().Format(time.RFC3339Nano), ID: last.ID}.Encode()
	}
	return res, nil
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
