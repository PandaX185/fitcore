package handlers

import (
	"github.com/gin-gonic/gin"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
)

type bookings struct{}

func (*bookings) CreateBooking(c *gin.Context) { notImplemented(c) }
func (*bookings) GetBooking(c *gin.Context, bookingId oapi.BookingID) {
	notImplemented(c)
}
func (*bookings) CancelBooking(c *gin.Context, bookingId oapi.BookingID) {
	notImplemented(c)
}
func (*bookings) ListClassBookings(c *gin.Context, classId oapi.ClassID) {
	notImplemented(c)
}
