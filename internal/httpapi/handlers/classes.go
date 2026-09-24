package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
	"github.com/PandaX185/fitcore/internal/modules/classes"
	"github.com/PandaX185/fitcore/internal/platform/httpx"
	"github.com/PandaX185/fitcore/internal/platform/telemetry"
)

// classService is the slice of the classes service the adapter consumes.
type classService interface {
	Get(ctx context.Context, id uuid.UUID) (*classes.Class, error)
	Create(ctx context.Context, branchID uuid.UUID, trainerID *uuid.UUID, name string, startsAt, endsAt time.Time, capacity int) (*classes.Class, error)
	Update(ctx context.Context, id uuid.UUID, patch classes.Patch) (*classes.Class, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, branchID *uuid.UUID, trainerID *uuid.UUID) ([]*classes.Class, error)
}

type classesHandler struct {
	svc     classService
	log     *slog.Logger
	metrics *telemetry.Metrics
}

func (h *classesHandler) ListClasses(c *gin.Context, params oapi.ListClassesParams) {
	var branchID, trainerID *uuid.UUID
	if params.BranchId != nil {
		id := uuid.UUID(*params.BranchId)
		branchID = &id
	}
	if params.TrainerId != nil {
		id := uuid.UUID(*params.TrainerId)
		trainerID = &id
	}
	cs, err := h.svc.List(c.Request.Context(), branchID, trainerID)
	if err != nil {
		h.fail(c, "listClasses", err)
		return
	}
	items := make([]oapi.Class, 0, len(cs))
	for _, cl := range cs {
		items = append(items, toClassResponse(cl))
	}
	httpx.JSON(c, http.StatusOK, items)
}

func (h *classesHandler) CreateClass(c *gin.Context) {
	var req oapi.ClassCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "classes", "createClass", http.StatusBadRequest, "invalid request body", err)
		return
	}
	cl, err := h.svc.Create(c.Request.Context(), uuid.UUID(req.BranchId), (*uuid.UUID)(req.TrainerId), req.Name, req.StartsAt, req.EndsAt, req.Capacity)
	if err != nil {
		h.fail(c, "createClass", err)
		return
	}
	httpx.JSON(c, http.StatusCreated, toClassResponse(cl))
}

func (h *classesHandler) GetClass(c *gin.Context, classId oapi.ClassID) {
	cl, err := h.svc.Get(c.Request.Context(), uuid.UUID(classId))
	if err != nil {
		h.fail(c, "getClass", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toClassResponse(cl))
}

func (h *classesHandler) UpdateClass(c *gin.Context, classId oapi.ClassID) {
	var req oapi.ClassUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "classes", "updateClass", http.StatusBadRequest, "invalid request body", err)
		return
	}
	patch := classes.Patch{
		Name:     req.Name,
		StartsAt: req.StartsAt,
		EndsAt:   req.EndsAt,
		Capacity: req.Capacity,
	}
	cl, err := h.svc.Update(c.Request.Context(), uuid.UUID(classId), patch)
	if err != nil {
		h.fail(c, "updateClass", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toClassResponse(cl))
}

func (h *classesHandler) DeleteClass(c *gin.Context, classId oapi.ClassID) {
	if err := h.svc.Delete(c.Request.Context(), uuid.UUID(classId)); err != nil {
		h.fail(c, "deleteClass", err)
		return
	}
	c.Status(http.StatusNoContent)
}

// fail maps classes sentinel errors to HTTP statuses.
func (h *classesHandler) fail(c *gin.Context, op string, err error) {
	status := http.StatusInternalServerError
	msg := "internal error"
	switch {
	case err == nil:
		return
	case errors.Is(err, classes.ErrNotFound):
		status = http.StatusNotFound
		msg = "class not found"
	case errors.Is(err, classes.ErrBranchNotFound):
		status = http.StatusNotFound
		msg = "branch not found"
	case errors.Is(err, classes.ErrTrainerNotFound):
		status = http.StatusNotFound
		msg = "trainer not found"
	case errors.Is(err, classes.ErrInvalidInput):
		status = http.StatusBadRequest
		msg = "invalid class"
	}
	httpx.Error(c, h.log, h.metrics, "classes", op, status, msg, err)
}

func toClassResponse(cl *classes.Class) oapi.Class {
	var trainerID *oapi.UUID
	if cl.TrainerID != nil {
		tr := oapi.UUID(*cl.TrainerID)
		trainerID = &tr
	}
	return oapi.Class{
		Id:        oapi.UUID(cl.ID),
		BranchId:  oapi.UUID(cl.BranchID),
		TrainerId: trainerID,
		Name:      cl.Name,
		StartsAt:  cl.StartsAt,
		EndsAt:    cl.EndsAt,
		Capacity:  cl.Capacity,
		CreatedAt: cl.CreatedAt,
		UpdatedAt: cl.UpdatedAt,
	}
}
