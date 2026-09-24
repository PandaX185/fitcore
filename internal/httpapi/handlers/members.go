package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
	"github.com/PandaX185/fitcore/internal/modules/members"
	"github.com/PandaX185/fitcore/internal/platform/httpx"
	"github.com/PandaX185/fitcore/internal/platform/telemetry"
)

// memberService is the slice of the members service the adapter consumes.
type memberService interface {
	Get(ctx context.Context, id uuid.UUID) (*members.Member, error)
	Create(ctx context.Context, branchID uuid.UUID, name, email, phone string) (*members.Member, error)
	Update(ctx context.Context, id uuid.UUID, patch members.Patch) (*members.Member, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context) ([]*members.Member, error)
}

type membersHandler struct {
	svc     memberService
	log     *slog.Logger
	metrics *telemetry.Metrics
}

func (h *membersHandler) ListMembers(c *gin.Context) {
	ms, err := h.svc.List(c.Request.Context())
	if err != nil {
		h.fail(c, "listMembers", err)
		return
	}
	items := make([]oapi.Member, 0, len(ms))
	for _, m := range ms {
		items = append(items, toMemberResponse(m))
	}
	httpx.JSON(c, http.StatusOK, items)
}

func (h *membersHandler) CreateMember(c *gin.Context) {
	var req oapi.MemberCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "members", "createMember", http.StatusBadRequest, "invalid request body", err)
		return
	}
	phone := ""
	if req.Phone != nil {
		phone = *req.Phone
	}
	m, err := h.svc.Create(c.Request.Context(), uuid.UUID(req.BranchId), req.Name, string(req.Email), phone)
	if err != nil {
		h.fail(c, "createMember", err)
		return
	}
	httpx.JSON(c, http.StatusCreated, toMemberResponse(m))
}

func (h *membersHandler) GetMember(c *gin.Context, memberId oapi.MemberID) {
	m, err := h.svc.Get(c.Request.Context(), uuid.UUID(memberId))
	if err != nil {
		h.fail(c, "getMember", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toMemberResponse(m))
}

func (h *membersHandler) UpdateMember(c *gin.Context, memberId oapi.MemberID) {
	var req oapi.MemberUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "members", "updateMember", http.StatusBadRequest, "invalid request body", err)
		return
	}
	patch := members.Patch{
		Name:   req.Name,
		Email:  (*string)(req.Email),
		Phone:  req.Phone,
		Status: (*members.Status)(req.Status),
	}
	m, err := h.svc.Update(c.Request.Context(), uuid.UUID(memberId), patch)
	if err != nil {
		h.fail(c, "updateMember", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toMemberResponse(m))
}

func (h *membersHandler) DeleteMember(c *gin.Context, memberId oapi.MemberID) {
	if err := h.svc.Delete(c.Request.Context(), uuid.UUID(memberId)); err != nil {
		h.fail(c, "deleteMember", err)
		return
	}
	c.Status(http.StatusNoContent)
}

// fail maps members sentinel errors to HTTP statuses and responds consistently.
// Members respond 400 (not 422) for invalid input and 409 for duplicate email.
func (h *membersHandler) fail(c *gin.Context, op string, err error) {
	status := http.StatusInternalServerError
	msg := "internal error"
	switch {
	case err == nil:
		return
	case errors.Is(err, members.ErrNotFound):
		status = http.StatusNotFound
		msg = "member not found"
	case errors.Is(err, members.ErrInvalidInput):
		status = http.StatusBadRequest
		msg = "invalid member"
	case errors.Is(err, members.ErrDuplicateEmail):
		status = http.StatusConflict
		msg = "email already in use"
	}
	httpx.Error(c, h.log, h.metrics, "members", op, status, msg, err)
}

func toMemberResponse(m *members.Member) oapi.Member {
	return oapi.Member{
		Id:        oapi.UUID(m.ID),
		BranchId:  oapi.UUID(m.BranchID),
		Name:      m.Name,
		Email:     openapi_types.Email(m.Email),
		Phone:     m.Phone,
		Status:    oapi.MemberStatus(m.Status),
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
