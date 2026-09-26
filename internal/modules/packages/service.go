package packages

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/paging"
)

// Service implements the membership-package business rules on the repo port.
type Service struct {
	repo PackageRepository
}

func NewService(repo PackageRepository) *Service {
	return &Service{repo: repo}
}

// Get returns the package with the given ID, mapping a missing row to ErrNotFound.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Package, error) {
	if id == uuid.Nil {
		return nil, ErrInvalid
	}
	return s.repo.GetByID(ctx, id)
}

// Create validates and persists a new package; new packages are active.
func (s *Service) Create(ctx context.Context, name string, durationDays int, priceCents int64, currency string) (*Package, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalid
	}
	currency, err := normalizeCurrency(currency)
	if err != nil {
		return nil, ErrInvalid
	}
	if durationDays <= 0 || priceCents < 0 {
		return nil, ErrInvalid
	}
	now := time.Now().UTC()
	p := &Package{
		ID:           uuid.New(),
		Name:         name,
		DurationDays: durationDays,
		PriceCents:   priceCents,
		Currency:     currency,
		Active:       true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Update applies a partial patch and returns the refreshed package.
func (s *Service) Update(ctx context.Context, id uuid.UUID, patch Patch) (*Package, error) {
	if id == uuid.Nil {
		return nil, ErrInvalid
	}
	if patch.Name != nil && strings.TrimSpace(*patch.Name) == "" {
		return nil, ErrInvalid
	}
	if patch.DurationDays != nil && *patch.DurationDays <= 0 {
		return nil, ErrInvalid
	}
	if patch.PriceCents != nil && *patch.PriceCents < 0 {
		return nil, ErrInvalid
	}
	if patch.Currency != nil {
		currency, err := normalizeCurrency(*patch.Currency)
		if err != nil {
			return nil, ErrInvalid
		}
		patch.Currency = &currency
	}
	if err := s.repo.Update(ctx, id, &patch); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// List returns one page of packages ordered by (name, id), with an opaque
// cursor for the next page when more rows remain.
func (s *Service) List(ctx context.Context, p ListParams) (*ListResult, error) {
	limit := paging.Limit(p.Limit)
	cursor, err := paging.DecodeCursor(p.Cursor)
	if err != nil {
		return nil, ErrInvalid
	}
	items, err := s.repo.List(ctx, &ListQuery{
		Limit:     limit + 1,
		AfterName: cursor.Key,
		AfterID:   cursor.ID,
	})
	if err != nil {
		return nil, err
	}
	res := &ListResult{Items: items}
	if len(items) > limit {
		res.Items = items[:limit]
		res.NextCursor = paging.Cursor{Key: items[limit-1].Name, ID: items[limit-1].ID}.Encode()
	}
	return res, nil
}

// normalizeCurrency uppercases and validates a 3-letter currency code.
func normalizeCurrency(code string) (string, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	if len(code) != 3 {
		return "", ErrInvalid
	}
	for _, r := range code {
		if r < 'A' || r > 'Z' {
			return "", ErrInvalid
		}
	}
	return code, nil
}
