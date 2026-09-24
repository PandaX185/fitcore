package members

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
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

// List returns all members ordered by (name, id).
func (s *Service) List(ctx context.Context) ([]*Member, error) {
	return s.repo.List(ctx)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
