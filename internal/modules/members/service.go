package members

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/paging"
)

// Service implements the member lifecycle business rules on the repo port.
type Service struct {
	repo MemberRepository
}

func NewService(repo MemberRepository) *Service {
	return &Service{repo: repo}
}

// Get returns the member with the given ID, mapping a missing row to ErrNotFound.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Member, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	return s.repo.GetByID(ctx, id)
}

// Create validates and persists a new member; default status is active.
func (s *Service) Create(ctx context.Context, branchID uuid.UUID, name, email, phone string) (*Member, error) {
	if branchID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	email = normalizeEmail(email)
	if strings.TrimSpace(name) == "" || email == "" || !strings.Contains(email, "@") {
		return nil, ErrInvalidInput
	}
	now := time.Now().UTC()
	m := &Member{
		ID:        uuid.New(),
		BranchID:  branchID,
		Name:      name,
		Email:     email,
		Phone:     phone,
		Status:    StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Update applies a partial patch and returns the refreshed member.
func (s *Service) Update(ctx context.Context, id uuid.UUID, patch Patch) (*Member, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	if patch.Name != nil && strings.TrimSpace(*patch.Name) == "" {
		return nil, ErrInvalidInput
	}
	if patch.Email != nil {
		normalized := normalizeEmail(*patch.Email)
		if normalized == "" || !strings.Contains(normalized, "@") {
			return nil, ErrInvalidInput
		}
		patch.Email = &normalized
	}
	if patch.Status != nil && *patch.Status != StatusActive && *patch.Status != StatusSuspended {
		return nil, ErrInvalidInput
	}
	if err := s.repo.Update(ctx, id, &patch); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// Delete removes a member by ID, mapping a missing row to ErrNotFound.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return ErrInvalidInput
	}
	return s.repo.Delete(ctx, id)
}

// List returns one page of members ordered by (name, id), with an opaque
// cursor for the next page when more rows remain.
func (s *Service) List(ctx context.Context, p ListParams) (*ListResult, error) {
	limit := paging.Limit(p.Limit)
	cursor, err := paging.DecodeCursor(p.Cursor)
	if err != nil {
		return nil, ErrInvalidInput
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

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
