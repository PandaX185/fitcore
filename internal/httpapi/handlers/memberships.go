package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
	"github.com/PandaX185/fitcore/internal/modules/memberships"
	"github.com/PandaX185/fitcore/internal/platform/httpx"
	"github.com/PandaX185/fitcore/internal/platform/telemetry"
)

// membershipService is the slice of the memberships service the adapter uses.
type membershipService interface {
	Get(ctx context.Context, id uuid.UUID) (*memberships.Membership, error)
	Create(ctx context.Context, memberID, packageID, branchID uuid.UUID) (*memberships.Membership, error)
	Update(ctx context.Context, id uuid.UUID, patch memberships.Patch) (*memberships.Membership, error)
	ListByMember(ctx context.Context, memberID uuid.UUID, p memberships.MemberListParams) (*memberships.MemberListResult, error)
}

type membershipsHandler struct {
	svc     membershipService
	log     *slog.Logger
	metrics *telemetry.Metrics
}

func (h *membershipsHandler) CreateMembership(c *gin.Context) {
	var req oapi.MembershipCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "memberships", "createMembership", http.StatusBadRequest, "invalid request body", err)
		return
	}
	m, err := h.svc.Create(c.Request.Context(), uuid.UUID(req.MemberId), uuid.UUID(req.PackageId), uuid.UUID(req.BranchId))
	if err != nil {
		h.fail(c, "createMembership", err)
		return
	}
	httpx.JSON(c, http.StatusCreated, toMembershipResponse(m))
}

func (h *membershipsHandler) GetMembership(c *gin.Context, membershipId oapi.MembershipID) {
	m, err := h.svc.Get(c.Request.Context(), uuid.UUID(membershipId))
	if err != nil {
		h.fail(c, "getMembership", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toMembershipResponse(m))
}

func (h *membershipsHandler) UpdateMembership(c *gin.Context, membershipId oapi.MembershipID) {
	var req oapi.MembershipUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "memberships", "updateMembership", http.StatusBadRequest, "invalid request body", err)
		return
	}
	patch := memberships.Patch{
		Status:    (*memberships.Status)(req.Status),
		ExpiresAt: req.ExpiresAt,
	}
	m, err := h.svc.Update(c.Request.Context(), uuid.UUID(membershipId), patch)
	if err != nil {
		h.fail(c, "updateMembership", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toMembershipResponse(m))
}

func (h *membershipsHandler) ListMemberMemberships(c *gin.Context, memberId oapi.MemberID, params oapi.ListMemberMembershipsParams) {
	p := memberships.MemberListParams{}
	if params.Limit != nil {
		p.Limit = *params.Limit
	}
	if params.Cursor != nil {
		p.Cursor = *params.Cursor
	}
	res, err := h.svc.ListByMember(c.Request.Context(), uuid.UUID(memberId), p)
	if err != nil {
		h.fail(c, "listMemberMemberships", err)
		return
	}
	items := make([]oapi.Membership, 0, len(res.Items))
	for _, m := range res.Items {
		items = append(items, toMembershipResponse(m))
	}
	page := oapi.MembershipPage{Items: items}
	if res.NextCursor != "" {
		page.NextCursor = &res.NextCursor
	}
	httpx.JSON(c, http.StatusOK, page)
}

// fail maps memberships sentinel errors to HTTP statuses: client input is a
// 400, a missing member or package is a 404.
func (h *membershipsHandler) fail(c *gin.Context, op string, err error) {
	status := http.StatusInternalServerError
	msg := "internal error"
	switch {
	case err == nil:
		return
	case errors.Is(err, memberships.ErrNotFound), errors.Is(err, memberships.ErrMemberNotFound), errors.Is(err, memberships.ErrPackageNotFound):
		status = http.StatusNotFound
		msg = "membership not found"
	case errors.Is(err, memberships.ErrInvalidInput):
		status = http.StatusBadRequest
		msg = "invalid membership"
	case errors.Is(err, memberships.ErrDuplicateActive):
		status = http.StatusConflict
		msg = "membership conflict"
	}
	httpx.Error(c, h.log, h.metrics, "memberships", op, status, msg, err)
}

func toMembershipResponse(m *memberships.Membership) oapi.Membership {
	return oapi.Membership{
		Id:        oapi.UUID(m.ID),
		MemberId:  oapi.UUID(m.MemberID),
		PackageId: oapi.UUID(m.PackageID),
		BranchId:  oapi.UUID(m.BranchID),
		StartsAt:  m.StartsAt,
		ExpiresAt: m.ExpiresAt,
		Status:    oapi.MembershipStatus(m.Status),
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
