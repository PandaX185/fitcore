package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
	"github.com/PandaX185/fitcore/internal/modules/auth"
	"github.com/PandaX185/fitcore/internal/modules/staff"
	"github.com/PandaX185/fitcore/internal/platform/httpx"
	"github.com/PandaX185/fitcore/internal/platform/telemetry"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// staffService is the slice of the staff service the adapter consumes.
type staffService interface {
	Get(ctx context.Context, id uuid.UUID) (*staff.Staff, error)
	Create(ctx context.Context, branchID uuid.UUID, name, email, phone string, permissions []auth.Permission) (*staff.Staff, error)
	Update(ctx context.Context, id uuid.UUID, patch staff.Patch) (*staff.Staff, error)
	ListByBranch(ctx context.Context, branchID uuid.UUID) ([]*staff.Staff, error)
}

type staffHandler struct {
	svc     staffService
	log     *slog.Logger
	metrics *telemetry.Metrics
}

func (h *staffHandler) ListBranchStaff(c *gin.Context, branchId oapi.BranchID) {
	staff, err := h.svc.ListByBranch(c.Request.Context(), uuid.UUID(branchId))
	if err != nil {
		h.fail(c, "listBranchStaff", err)
		return
	}
	items := make([]oapi.Staff, 0, len(staff))
	for _, s := range staff {
		items = append(items, toStaffResponse(s))
	}
	httpx.JSON(c, http.StatusOK, items)
}

func (h *staffHandler) CreateStaff(c *gin.Context) {
	var req oapi.StaffCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "staff", "createStaff", http.StatusBadRequest, "invalid request body", err)
		return
	}
	s, err := h.svc.Create(c.Request.Context(), uuid.UUID(req.BranchId), req.Name, string(req.Email), derefString(req.Phone), permissionListPtr(req.Permissions))
	if err != nil {
		h.fail(c, "createStaff", err)
		return
	}
	httpx.JSON(c, http.StatusCreated, toStaffResponse(s))
}

func (h *staffHandler) GetStaff(c *gin.Context, staffId oapi.StaffID) {
	s, err := h.svc.Get(c.Request.Context(), uuid.UUID(staffId))
	if err != nil {
		h.fail(c, "getStaff", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toStaffResponse(s))
}

func (h *staffHandler) UpdateStaff(c *gin.Context, staffId oapi.StaffID) {
	var req oapi.StaffUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "staff", "updateStaff", http.StatusBadRequest, "invalid request body", err)
		return
	}
	perms := permissionListPtr(req.Permissions)
	patch := staff.Patch{
		Name:        req.Name,
		Email:       emailPtr(req.Email),
		Phone:       req.Phone,
		Permissions: &perms,
		Active:      req.Active,
	}
	s, err := h.svc.Update(c.Request.Context(), uuid.UUID(staffId), patch)
	if err != nil {
		h.fail(c, "updateStaff", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toStaffResponse(s))
}

// fail maps staff sentinel errors to HTTP statuses.
func (h *staffHandler) fail(c *gin.Context, op string, err error) {
	status := http.StatusInternalServerError
	msg := "internal error"
	switch {
	case err == nil:
		return
	case errors.Is(err, staff.ErrNotFound):
		status = http.StatusNotFound
		msg = "staff not found"
	case errors.Is(err, staff.ErrBranchNotFound):
		status = http.StatusNotFound
		msg = "branch not found"
	case errors.Is(err, staff.ErrInvalidInput):
		status = http.StatusBadRequest
		msg = "invalid staff"
	case errors.Is(err, staff.ErrDuplicateEmail):
		status = http.StatusBadRequest
		msg = "email already in use"
	}
	httpx.Error(c, h.log, h.metrics, "staff", op, status, msg, err)
}

func toStaffResponse(s *staff.Staff) oapi.Staff {
	permissions := make([]string, 0, len(s.Permissions))
	for _, p := range s.Permissions {
		permissions = append(permissions, string(p))
	}
	return oapi.Staff{
		Id:          oapi.UUID(s.ID),
		BranchId:    oapi.UUID(s.BranchID),
		Name:        s.Name,
		Email:       openapi_types.Email(s.Email),
		Phone:       s.Phone,
		Permissions: permissions,
		Active:      s.Active,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

// permissionListPtr converts a request permission list to the domain type,
// preserving nil.
func permissionListPtr(perms *[]string) []auth.Permission {
	if perms == nil {
		return nil
	}
	out := make([]auth.Permission, 0, len(*perms))
	for _, p := range *perms {
		out = append(out, auth.Permission(p))
	}
	return out
}

// derefString dereferences a pointer string, falling back to "".
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// emailPtr converts a request email pointer to a domain string pointer,
// preserving nil.
func emailPtr(e *openapi_types.Email) *string {
	if e == nil {
		return nil
	}
	val := string(*e)
	return &val
}
