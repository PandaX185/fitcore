package branches

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/paging"
)

// Service implements the branches business rules on top of the repository port.
type Service struct {
	repo BranchRepository
}

func NewService(repo BranchRepository) *Service {
	return &Service{repo: repo}
}

// Get returns the branch with the given ID, mapping a missing row to ErrNotFound.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Branch, error) {
	if id == uuid.Nil {
		return nil, ErrInvalid
	}
	return s.repo.GetByID(ctx, id)
}

// Create validates and persists a new branch.
func (s *Service) Create(ctx context.Context, name, address string, latitude, longitude float64) (*Branch, error) {
	if strings.TrimSpace(name) == "" {
		return nil, ErrInvalid
	}
	if err := validateCoord(latitude, longitude); err != nil {
		return nil, ErrInvalid
	}
	now := time.Now().UTC()
	b := &Branch{
		ID:        uuid.New(),
		Name:      name,
		Address:   address,
		Latitude:  latitude,
		Longitude: longitude,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, b); err != nil {
		return nil, err
	}
	return b, nil
}

// Update applies a partial patch and returns the refreshed branch.
func (s *Service) Update(ctx context.Context, id uuid.UUID, patch Patch) (*Branch, error) {
	if id == uuid.Nil {
		return nil, ErrInvalid
	}
	if patch.Name != nil && strings.TrimSpace(*patch.Name) == "" {
		return nil, ErrInvalid
	}
	if (patch.Latitude != nil) != (patch.Longitude != nil) {
		return nil, ErrInvalid
	}
	if patch.Latitude != nil {
		if err := validateCoord(*patch.Latitude, *patch.Longitude); err != nil {
			return nil, ErrInvalid
		}
	}
	if err := s.repo.Update(ctx, id, &patch); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// List returns one page of branches ordered by (name, id), with an opaque
// cursor for the next page when more rows remain.
func (s *Service) List(ctx context.Context, p ListParams) (*ListResult, error) {
	limit := paging.Limit(p.Limit)
	cursor, err := paging.DecodeCursor(p.Cursor)
	if err != nil {
		return nil, ErrInvalid
	}
	q := &ListQuery{
		Query:     strings.TrimSpace(p.Query),
		Limit:     limit + 1,
		AfterName: cursor.Key,
		AfterID:   cursor.ID,
	}
	items, err := s.repo.List(ctx, q)
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

func validateCoord(latitude, longitude float64) error {
	if latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
		return ErrInvalid
	}
	return nil
}
