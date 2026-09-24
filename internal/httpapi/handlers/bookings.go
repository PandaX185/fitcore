package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
	"github.com/PandaX185/fitcore/internal/modules/bookings"
	"github.com/PandaX185/fitcore/internal/platform/httpx"
	"github.com/PandaX185/fitcore/internal/platform/telemetry"
)

// bookingService is the slice of the bookings service the adapter consumes.
type bookingService interface {
	Get(ctx context.Context, id uuid.UUID) (*bookings.Booking, error)
	Create(ctx context.Context, classID, memberID uuid.UUID) (*bookings.Booking, error)
	Cancel(ctx context.Context, id uuid.UUID) (*bookings.Booking, error)
	ListByClass(ctx context.Context, classID uuid.UUID) ([]*bookings.Booking, error)
}

type bookingsHandler struct {
	svc     bookingService
	log     *slog.Logger
	metrics *telemetry.Metrics
}

func (h *bookingsHandler) CreateBooking(c *gin.Context) {
	var req oapi.BookingCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, h.log, h.metrics, "bookings", "createBooking", http.StatusBadRequest, "invalid request body", err)
		return
	}
	b, err := h.svc.Create(c.Request.Context(), uuid.UUID(req.ClassId), uuid.UUID(req.MemberId))
	if err != nil {
		h.fail(c, "createBooking", err)
		return
	}
	httpx.JSON(c, http.StatusCreated, toBookingResponse(b))
}

func (h *bookingsHandler) GetBooking(c *gin.Context, bookingId oapi.BookingID) {
	b, err := h.svc.Get(c.Request.Context(), uuid.UUID(bookingId))
	if err != nil {
		h.fail(c, "getBooking", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toBookingResponse(b))
}

func (h *bookingsHandler) CancelBooking(c *gin.Context, bookingId oapi.BookingID) {
	b, err := h.svc.Cancel(c.Request.Context(), uuid.UUID(bookingId))
	if err != nil {
		h.fail(c, "cancelBooking", err)
		return
	}
	httpx.JSON(c, http.StatusOK, toBookingResponse(b))
}

func (h *bookingsHandler) ListClassBookings(c *gin.Context, classId oapi.ClassID) {
	bs, err := h.svc.ListByClass(c.Request.Context(), uuid.UUID(classId))
	if err != nil {
		h.fail(c, "listClassBookings", err)
		return
	}
	items := make([]oapi.Booking, 0, len(bs))
	for _, b := range bs {
		items = append(items, toBookingResponse(b))
	}
	httpx.JSON(c, http.StatusOK, items)
}

// fail maps bookings sentinel errors to HTTP statuses. Capacity, duplicate
// bookings and stale classes are 409 conflicts per the spec.
func (h *bookingsHandler) fail(c *gin.Context, op string, err error) {
	status := http.StatusInternalServerError
	msg := "internal error"
	switch {
	case err == nil:
		return
	case errors.Is(err, bookings.ErrNotFound):
		status = http.StatusNotFound
		msg = "booking not found"
	case errors.Is(err, bookings.ErrClassNotFound), errors.Is(err, bookings.ErrMemberNotFound):
		status = http.StatusNotFound
		msg = "class or member not found"
	case errors.Is(err, bookings.ErrInvalidInput):
		status = http.StatusBadRequest
		msg = "invalid booking"
	case errors.Is(err, bookings.ErrClassFull), errors.Is(err, bookings.ErrDuplicate):
		status = http.StatusConflict
		msg = "booking conflict"
	}
	httpx.Error(c, h.log, h.metrics, "bookings", op, status, msg, err)
}

func toBookingResponse(b *bookings.Booking) oapi.Booking {
	var cancelledAt *oapi.Timestamp
	if b.CancelledAt != nil {
		ct := oapi.Timestamp(*b.CancelledAt)
		cancelledAt = &ct
	}
	return oapi.Booking{
		Id:          oapi.UUID(b.ID),
		ClassId:     oapi.UUID(b.ClassID),
		MemberId:    oapi.UUID(b.MemberID),
		Status:      oapi.BookingStatus(b.Status),
		BookedAt:    b.BookedAt,
		CancelledAt: cancelledAt,
	}
}
