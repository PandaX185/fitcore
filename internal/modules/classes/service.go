package classes

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/branches"
	"github.com/PandaX185/fitcore/internal/modules/trainers"
)

// Service implements the class-scheduling business rules on the ports.
type Service struct {
	repo     ClassRepository
	branches BranchReader
	trainers TrainerReader
}

func NewService(repo ClassRepository, branches BranchReader, trainers TrainerReader) *Service {
	return &Service{repo: repo, branches: branches, trainers: trainers}
}

// Get returns the class with the given ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Class, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	return s.repo.GetByID(ctx, id)
}

// Create schedules a new class. An optional trainer must exist and belong to
// the same branch; the class must fit its branch and end after it starts.
func (s *Service) Create(ctx context.Context, branchID uuid.UUID, trainerID *uuid.UUID, name string, startsAt, endsAt time.Time, capacity int) (*Class, error) {
	if branchID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	if strings.TrimSpace(name) == "" || capacity <= 0 || !endsAt.After(startsAt) {
		return nil, ErrInvalidInput
	}
	b, err := s.branches.Get(ctx, branchID)
	if err != nil {
		if errors.Is(err, branches.ErrNotFound) {
			return nil, ErrBranchNotFound
		}
		return nil, err
	}
	if trainerID != nil && *trainerID != uuid.Nil {
		t, err := s.trainers.Get(ctx, *trainerID)
		if err != nil {
			if errors.Is(err, trainers.ErrNotFound) {
				return nil, ErrTrainerNotFound
			}
			return nil, err
		}
		if t.BranchID != b.ID {
			return nil, ErrInvalidInput
		}
	}
	now := time.Now().UTC()
	c := &Class{
		ID:        uuid.New(),
		BranchID:  branchID,
		TrainerID: trainerID,
		Name:      strings.TrimSpace(name),
		StartsAt:  startsAt,
		EndsAt:    endsAt,
		Capacity:  capacity,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Update applies a partial patch and returns the refreshed class.
func (s *Service) Update(ctx context.Context, id uuid.UUID, patch Patch) (*Class, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	starts := existing.StartsAt
	if patch.StartsAt != nil {
		starts = *patch.StartsAt
	}
	ends := existing.EndsAt
	if patch.EndsAt != nil {
		ends = *patch.EndsAt
	}
	if !ends.After(starts) {
		return nil, ErrInvalidInput
	}
	if patch.Capacity != nil && *patch.Capacity <= 0 {
		return nil, ErrInvalidInput
	}
	if patch.Name != nil && strings.TrimSpace(*patch.Name) == "" {
		return nil, ErrInvalidInput
	}
	if err := s.repo.Update(ctx, id, &patch); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// Delete removes the class with the given ID.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return ErrInvalidInput
	}
	return s.repo.Delete(ctx, id)
}

// List returns classes, optionally filtered by branch and trainer.
func (s *Service) List(ctx context.Context, branchID *uuid.UUID, trainerID *uuid.UUID) ([]*Class, error) {
	if branchID != nil && *branchID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	return s.repo.List(ctx, branchID, trainerID)
}
