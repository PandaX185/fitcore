package staff

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/auth"
	"github.com/PandaX185/fitcore/internal/modules/branches"
	"github.com/PandaX185/fitcore/internal/paging"
)

// Service implements the staff lifecycle business rules on the ports.
type Service struct {
	repo     StaffRepository
	branches BranchReader
}

func NewService(repo StaffRepository, branches BranchReader) *Service {
	return &Service{repo: repo, branches: branches}
}

// Get returns the staff member with the given ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Staff, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	return s.repo.GetByID(ctx, id)
}

// Create persists a new staff member. Grants must be known permissions; the
// member starts active. Passwords are provisioned out-of-band (cmd/set-password).
func (s *Service) Create(ctx context.Context, branchID uuid.UUID, name, email, phone string, permissions []auth.Permission) (*Staff, error) {
	if branchID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	email = normalizeEmail(email)
	if strings.TrimSpace(name) == "" || email == "" || !strings.Contains(email, "@") {
		return nil, ErrInvalidInput
	}
	if permissions != nil && !auth.Known(permissions) {
		return nil, ErrInvalidInput
	}
	if _, err := s.branches.Get(ctx, branchID); err != nil {
		if errors.Is(err, branches.ErrNotFound) {
			return nil, ErrBranchNotFound
		}
		return nil, err
	}
	now := time.Now().UTC()
	st := &Staff{
		ID:          uuid.New(),
		BranchID:    branchID,
		Name:        name,
		Email:       email,
		Phone:       phone,
		Permissions: permissions,
		Active:      true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Create(ctx, st); err != nil {
		return nil, err
	}
	return st, nil
}

// Update applies a partial patch and returns the refreshed staff member.
func (s *Service) Update(ctx context.Context, id uuid.UUID, patch Patch) (*Staff, error) {
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
	if patch.Permissions != nil && !auth.Known(*patch.Permissions) {
		return nil, ErrInvalidInput
	}
	if err := s.repo.Update(ctx, id, &patch); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// ListByBranch returns one page of the staff assigned to a branch ordered
// by (name, id), with an opaque cursor for the next page when more rows
// remain.
func (s *Service) ListByBranch(ctx context.Context, branchID uuid.UUID, p BranchListParams) (*BranchListResult, error) {
	if branchID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	limit := paging.Limit(p.Limit)
	cursor, err := paging.DecodeCursor(p.Cursor)
	if err != nil {
		return nil, ErrInvalidInput
	}
	items, err := s.repo.ListByBranch(ctx, &BranchListQuery{
		BranchID:  branchID,
		Limit:     limit + 1,
		AfterName: cursor.Key,
		AfterID:   cursor.ID,
	})
	if err != nil {
		return nil, err
	}
	res := &BranchListResult{Items: items}
	if len(items) > limit {
		res.Items = items[:limit]
		res.NextCursor = paging.Cursor{Key: items[limit-1].Name, ID: items[limit-1].ID}.Encode()
	}
	return res, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
