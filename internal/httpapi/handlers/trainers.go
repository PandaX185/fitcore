package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
	"github.com/PandaX185/fitcore/internal/modules/trainers"
	"github.com/PandaX185/fitcore/internal/platform/httpx"
	"github.com/PandaX185/fitcore/internal/platform/telemetry"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// trainerService is the slice of the trainers service the adapter consumes.
type trainerService interface {
	Get(ctx context.Context, id uuid.UUID) (*trainers.Trainer, error)
	Create(ctx context.Context, branchID uuid.UUID, name, email, phone string) (*trainers.Trainer, error)
	Update(ctx context.Context, id uuid.UUID, patch trainers.Patch) (*trainers.Trainer, error)
	ListByBranch(ctx context.Context, branchID uuid.UUID) ([]*trainers.Trainer, error)
}

type trainersHandler struct {
	svc     trainerService
	log     *slog.Logger
	metrics *telemetry.Metrics
}

func (h *trainersHandler) ListBranchTrainers(c *gin.Context, branchId oapi.BranchID) {
	ts, err := h.svc.ListByBranch(c.Request.Context(), uuid.UUID(branchId))
	if err != nil {
		h.fail(c, "listBranchTrainers", err)
		return
	}
	items := make([]oapi.Trainer, 0, len(ts))
	for _, t := range ts {
		items = append(items, toTrainerResponse(t))
	}
	httpx.JSON(c, http.StatusOK, items)
}

func (h *trainersHandler) CreateTrainer(c *gin.Context) {
	var req oapi.TrainerCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "trainers", "createTrainer", http.StatusBadRequest, "invalid request body", err)
		return
	}
	t, err := h.svc.Create(c.Request.Context(), uuid.UUID(req.BranchId), req.Name, string(req.Email), derefString(req.Phone))
	if err != nil {
		h.fail(c, "createTrainer", err)
		return
	}
	httpx.JSON(c, http.StatusCreated, toTrainerResponse(t))
}

func (h *trainersHandler) GetTrainer(c *gin.Context, trainerId oapi.TrainerID) {
	t, err := h.svc.Get(c.Request.Context(), uuid.UUID(trainerId))
	if err != nil {
		h.fail(c, "getTrainer", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toTrainerResponse(t))
}

func (h *trainersHandler) UpdateTrainer(c *gin.Context, trainerId oapi.TrainerID) {
	var req oapi.TrainerUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "trainers", "updateTrainer", http.StatusBadRequest, "invalid request body", err)
		return
	}
	patch := trainers.Patch{
		Name:   req.Name,
		Email:  emailPtr(req.Email),
		Phone:  req.Phone,
		Active: req.Active,
	}
	t, err := h.svc.Update(c.Request.Context(), uuid.UUID(trainerId), patch)
	if err != nil {
		h.fail(c, "updateTrainer", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toTrainerResponse(t))
}

// fail maps trainers sentinel errors to HTTP statuses.
func (h *trainersHandler) fail(c *gin.Context, op string, err error) {
	status := http.StatusInternalServerError
	msg := "internal error"
	switch {
	case err == nil:
		return
	case errors.Is(err, trainers.ErrNotFound):
		status = http.StatusNotFound
		msg = "trainer not found"
	case errors.Is(err, trainers.ErrBranchNotFound):
		status = http.StatusNotFound
		msg = "branch not found"
	case errors.Is(err, trainers.ErrInvalidInput):
		status = http.StatusBadRequest
		msg = "invalid trainer"
	case errors.Is(err, trainers.ErrDuplicateEmail):
		status = http.StatusBadRequest
		msg = "email already in use"
	}
	httpx.Error(c, h.log, h.metrics, "trainers", op, status, msg, err)
}

func toTrainerResponse(t *trainers.Trainer) oapi.Trainer {
	return oapi.Trainer{
		Id:        oapi.UUID(t.ID),
		BranchId:  oapi.UUID(t.BranchID),
		Name:      t.Name,
		Email:     openapi_types.Email(t.Email),
		Phone:     t.Phone,
		Active:    t.Active,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}
