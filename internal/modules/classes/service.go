package classes

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/branches"
	"github.com/PandaX185/fitcore/internal/modules/trainers"
	"github.com/PandaX185/fitcore/internal/paging"
	"github.com/PandaX185/fitcore/internal/transact"
)

// Service implements the class-scheduling business rules on the ports.
type Service struct {
	repo      ClassRepository
	branches  BranchReader
	trainers  TrainerReader
	occupancy ClassOccupancyReader
	tx        transact.Transactor
}

func NewService(repo ClassRepository, branches BranchReader, trainers TrainerReader, occupancy ClassOccupancyReader, tx transact.Transactor) *Service {
	return &Service{repo: repo, branches: branches, trainers: trainers, occupancy: occupancy, tx: tx}
}

// Get returns the class with the given ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Class, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	return s.repo.GetByID(ctx, id)
}

// GetForUpdate returns the class row locked (SELECT ... FOR UPDATE) for the
// bookings capacity protocol. It satisfies the bookings ClassReader port.
func (s *Service) GetForUpdate(ctx context.Context, id uuid.UUID) (*Class, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	return s.repo.GetForUpdate(ctx, id)
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

// Update applies a partial patch and returns the refreshed class. Shrinking
// capacity below the live booking count is rejected with ErrInvalidInput
// (400); the occupancy read and the write run inside one transaction.
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
	var updated *Class
	err = s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if patch.Capacity != nil {
			active, err := s.occupancy.CountActiveByClass(txCtx, id)
			if err != nil {
				return err
			}
			if *patch.Capacity < active {
				return ErrInvalidInput
			}
		}
		if err := s.repo.Update(txCtx, id, &patch); err != nil {
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

// Delete removes the class with the given ID, refusing with ErrHasBookings
// (409) while live bookings reference it.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return ErrInvalidInput
	}
	return s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		active, err := s.occupancy.CountActiveByClass(txCtx, id)
		if err != nil {
			return err
		}
		if active > 0 {
			return ErrHasBookings
		}
		return s.repo.Delete(txCtx, id)
	})
}

// List returns one page of classes ordered by (starts_at, id),
// optionally filtered by branch and trainer, with an opaque cursor for the
// next page when more rows remain.
func (s *Service) List(ctx context.Context, p ListParams) (*ListResult, error) {
	if p.BranchID != nil && *p.BranchID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	if p.TrainerID != nil && *p.TrainerID == uuid.Nil {
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
	items, err := s.repo.List(ctx, &ListQuery{
		BranchID:      p.BranchID,
		TrainerID:     p.TrainerID,
		Limit:         limit + 1,
		AfterStartsAt: cursor.Key,
		AfterID:       cursor.ID,
	})
	if err != nil {
		return nil, err
	}
	res := &ListResult{Items: items}
	if len(items) > limit {
		res.Items = items[:limit]
		last := items[limit-1]
		res.NextCursor = paging.Cursor{Key: last.StartsAt.UTC().Format(time.RFC3339Nano), ID: last.ID}.Encode()
	}
	return res, nil
}
