package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
	"github.com/PandaX185/fitcore/internal/modules/attendance"
	"github.com/PandaX185/fitcore/internal/platform/httpx"
	"github.com/PandaX185/fitcore/internal/platform/telemetry"
)

// attendanceService is the slice of the attendance service the adapter consumes.
type attendanceService interface {
	Get(ctx context.Context, id uuid.UUID) (*attendance.Attendance, error)
	CheckIn(ctx context.Context, memberID, branchID uuid.UUID) (*attendance.Attendance, error)
	CheckOut(ctx context.Context, memberID uuid.UUID) (*attendance.Attendance, error)
	ListByMember(ctx context.Context, memberID uuid.UUID) ([]*attendance.Attendance, error)
}

type attendanceHandler struct {
	svc     attendanceService
	log     *slog.Logger
	metrics *telemetry.Metrics
}

func (h *attendanceHandler) CheckIn(c *gin.Context) {
	var req oapi.AttendanceCheckInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "attendance", "checkIn", http.StatusBadRequest, "invalid request body", err)
		return
	}
	a, err := h.svc.CheckIn(c.Request.Context(), uuid.UUID(req.MemberId), uuid.UUID(req.BranchId))
	if err != nil {
		h.fail(c, "checkIn", err)
		return
	}
	httpx.JSON(c, http.StatusCreated, toAttendanceResponse(a))
}

func (h *attendanceHandler) CheckOut(c *gin.Context) {
	var req oapi.AttendanceCheckOutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "attendance", "checkOut", http.StatusBadRequest, "invalid request body", err)
		return
	}
	a, err := h.svc.CheckOut(c.Request.Context(), uuid.UUID(req.MemberId))
	if err != nil {
		h.fail(c, "checkOut", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toAttendanceResponse(a))
}

func (h *attendanceHandler) GetAttendance(c *gin.Context, attendanceId oapi.AttendanceID) {
	a, err := h.svc.Get(c.Request.Context(), uuid.UUID(attendanceId))
	if err != nil {
		h.fail(c, "getAttendance", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toAttendanceResponse(a))
}

func (h *attendanceHandler) ListMemberAttendance(c *gin.Context, memberId oapi.MemberID) {
	as, err := h.svc.ListByMember(c.Request.Context(), uuid.UUID(memberId))
	if err != nil {
		h.fail(c, "listMemberAttendance", err)
		return
	}
	items := make([]oapi.Attendance, 0, len(as))
	for _, a := range as {
		items = append(items, toAttendanceResponse(a))
	}
	httpx.JSON(c, http.StatusOK, items)
}

// fail maps attendance sentinel errors to HTTP statuses.
func (h *attendanceHandler) fail(c *gin.Context, op string, err error) {
	status := http.StatusInternalServerError
	msg := "internal error"
	switch {
	case err == nil:
		return
	case errors.Is(err, attendance.ErrNotFound), errors.Is(err, attendance.ErrMemberNotFound), errors.Is(err, attendance.ErrNoActiveMembership), errors.Is(err, attendance.ErrNoOpenRecord):
		status = http.StatusNotFound
		msg = "attendance record not found"
	case errors.Is(err, attendance.ErrInvalidInput):
		status = http.StatusBadRequest
		msg = "invalid attendance"
	case errors.Is(err, attendance.ErrAlreadyCheckedIn), errors.Is(err, attendance.ErrAlreadyCheckedOut):
		status = http.StatusConflict
		msg = "attendance conflict"
	}
	httpx.Error(c, h.log, h.metrics, "attendance", op, status, msg, err)
}

func toAttendanceResponse(a *attendance.Attendance) oapi.Attendance {
	var checkedOutAt *oapi.Timestamp
	if a.CheckedOutAt != nil {
		ct := oapi.Timestamp(*a.CheckedOutAt)
		checkedOutAt = &ct
	}
	var membershipID *oapi.UUID
	if a.MembershipID != uuid.Nil {
		mid := oapi.UUID(a.MembershipID)
		membershipID = &mid
	}
	return oapi.Attendance{
		Id:           oapi.UUID(a.ID),
		MemberId:     oapi.UUID(a.MemberID),
		BranchId:     oapi.UUID(a.BranchID),
		MembershipId: membershipID,
		CheckedInAt:  a.CheckedInAt,
		CheckedOutAt: checkedOutAt,
		CreatedAt:    a.CreatedAt,
	}
}
