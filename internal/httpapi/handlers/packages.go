package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
	"github.com/PandaX185/fitcore/internal/modules/packages"
	"github.com/PandaX185/fitcore/internal/platform/httpx"
	"github.com/PandaX185/fitcore/internal/platform/telemetry"
)

// packageService is the slice of the packages service the adapter consumes.
type packageService interface {
	Get(ctx context.Context, id uuid.UUID) (*packages.Package, error)
	Create(ctx context.Context, name string, durationDays int, priceCents int64, currency string) (*packages.Package, error)
	Update(ctx context.Context, id uuid.UUID, patch packages.Patch) (*packages.Package, error)
	List(ctx context.Context) ([]*packages.Package, error)
}

type packagesHandler struct {
	svc     packageService
	log     *slog.Logger
	metrics *telemetry.Metrics
}

func (h *packagesHandler) ListPackages(c *gin.Context) {
	ps, err := h.svc.List(c.Request.Context())
	if err != nil {
		h.fail(c, "listPackages", err)
		return
	}
	items := make([]oapi.Package, 0, len(ps))
	for _, p := range ps {
		items = append(items, toPackageResponse(p))
	}
	httpx.JSON(c, http.StatusOK, items)
}

func (h *packagesHandler) CreatePackage(c *gin.Context) {
	var req oapi.PackageCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "packages", "createPackage", http.StatusBadRequest, "invalid request body", err)
		return
	}
	p, err := h.svc.Create(c.Request.Context(), req.Name, req.DurationDays, req.PriceCents, req.Currency)
	if err != nil {
		h.fail(c, "createPackage", err)
		return
	}
	httpx.JSON(c, http.StatusCreated, toPackageResponse(p))
}

func (h *packagesHandler) GetPackage(c *gin.Context, packageId oapi.PackageID) {
	p, err := h.svc.Get(c.Request.Context(), uuid.UUID(packageId))
	if err != nil {
		h.fail(c, "getPackage", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toPackageResponse(p))
}

func (h *packagesHandler) UpdatePackage(c *gin.Context, packageId oapi.PackageID) {
	var req oapi.PackageUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "packages", "updatePackage", http.StatusBadRequest, "invalid request body", err)
		return
	}
	patch := packages.Patch{
		Name:         req.Name,
		DurationDays: req.DurationDays,
		PriceCents:   req.PriceCents,
		Currency:     req.Currency,
		Active:       req.Active,
	}
	p, err := h.svc.Update(c.Request.Context(), uuid.UUID(packageId), patch)
	if err != nil {
		h.fail(c, "updatePackage", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toPackageResponse(p))
}

// fail maps packages sentinel errors to HTTP statuses. Packages use 400 for
// client input problems (matching the spec's BadRequest response).
func (h *packagesHandler) fail(c *gin.Context, op string, err error) {
	status := http.StatusInternalServerError
	msg := "internal error"
	switch {
	case err == nil:
		return
	case errors.Is(err, packages.ErrNotFound):
		status = http.StatusNotFound
		msg = "package not found"
	case errors.Is(err, packages.ErrInvalid):
		status = http.StatusBadRequest
		msg = "invalid package"
	case errors.Is(err, packages.ErrDuplicateName):
		status = http.StatusBadRequest
		msg = "package name already in use"
	}
	httpx.Error(c, h.log, h.metrics, "packages", op, status, msg, err)
}

func toPackageResponse(p *packages.Package) oapi.Package {
	return oapi.Package{
		Id:           oapi.UUID(p.ID),
		Name:         p.Name,
		DurationDays: p.DurationDays,
		PriceCents:   p.PriceCents,
		Currency:     p.Currency,
		Active:       p.Active,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}
