package handlers

import (
	"github.com/gin-gonic/gin"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
)

type attendance struct{}

func (*attendance) CheckIn(c *gin.Context)  { notImplemented(c) }
func (*attendance) CheckOut(c *gin.Context) { notImplemented(c) }
func (*attendance) GetAttendance(c *gin.Context, attendanceId oapi.AttendanceID) {
	notImplemented(c)
}
func (*attendance) ListMemberAttendance(c *gin.Context, memberId oapi.MemberID) {
	notImplemented(c)
}
