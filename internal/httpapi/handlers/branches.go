package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
	"github.com/PandaX185/fitcore/internal/modules/branches"
	"github.com/PandaX185/fitcore/internal/platform/httpx"
	"github.com/PandaX185/fitcore/internal/platform/telemetry"
)

// branchService is the slice of the branches service the adapter consumes.
type branchService interface {
	Get(ctx context.Context, id uuid.UUID) (*branches.Branch, error)
	Create(ctx context.Context, name, address string, latitude, longitude float64) (*branches.Branch, error)
	Update(ctx context.Context, id uuid.UUID, patch branches.Patch) (*branches.Branch, error)
	List(ctx context.Context, p branches.ListParams) (*branches.ListResult, error)
}

type branchesHandler struct {
	svc     branchService
	log     *slog.Logger
	metrics *telemetry.Metrics
}

func (h *branchesHandler) ListBranches(c *gin.Context, params oapi.ListBranchesParams) {
	p := branches.ListParams{}
	if params.Q != nil {
		p.Query = *params.Q
	}
	if params.Limit != nil {
		p.Limit = *params.Limit
	}
	if params.Cursor != nil {
		p.Cursor = *params.Cursor
	}

	res, err := h.svc.List(c.Request.Context(), p)
	if err != nil {
		h.fail(c, "listBranches", err)
		return
	}
	items := make([]oapi.Branch, 0, len(res.Items))
	for _, b := range res.Items {
		items = append(items, toBranchResponse(b))
	}
	page := oapi.BranchPage{Items: items}
	if res.NextCursor != "" {
		page.NextCursor = &res.NextCursor
	}
	httpx.JSON(c, http.StatusOK, page)
}

func (h *branchesHandler) CreateBranch(c *gin.Context) {
	var req oapi.BranchCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "branches", "createBranch", http.StatusBadRequest, "invalid request body", err)
		return
	}

	b, err := h.svc.Create(c.Request.Context(), req.Name, req.Address, req.Latitude, req.Longitude)
	if err != nil {
		h.fail(c, "createBranch", err)
		return
	}
	httpx.JSON(c, http.StatusCreated, toBranchResponse(b))
}

func (h *branchesHandler) GetBranch(c *gin.Context, branchId oapi.BranchID) {
	b, err := h.svc.Get(c.Request.Context(), uuid.UUID(branchId))
	if err != nil {
		h.fail(c, "getBranch", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toBranchResponse(b))
}

func (h *branchesHandler) UpdateBranch(c *gin.Context, branchId oapi.BranchID) {
	var req oapi.BranchUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "branches", "updateBranch", http.StatusBadRequest, "invalid request body", err)
		return
	}

	patch := branches.Patch{
		Name:      req.Name,
		Address:   req.Address,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
	}
	b, err := h.svc.Update(c.Request.Context(), uuid.UUID(branchId), patch)
	if err != nil {
		h.fail(c, "updateBranch", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toBranchResponse(b))
}

// fail maps branches sentinel errors to HTTP statuses and responds consistently.
func (h *branchesHandler) fail(c *gin.Context, op string, err error) {
	status := httpx.StatusFor(err, branches.ErrNotFound, branches.ErrInvalid, nil)
	msg := "internal error"
	switch status {
	case http.StatusNotFound:
		msg = "branch not found"
	case http.StatusUnprocessableEntity:
		msg = "invalid branch"
	}
	httpx.Error(c, h.log, h.metrics, "branches", op, status, msg, err)
}

func toBranchResponse(b *branches.Branch) oapi.Branch {
	return oapi.Branch{
		Id:        oapi.UUID(b.ID),
		Name:      b.Name,
		Address:   b.Address,
		Latitude:  b.Latitude,
		Longitude: b.Longitude,
		CreatedAt: b.CreatedAt,
		UpdatedAt: b.UpdatedAt,
	}
}
