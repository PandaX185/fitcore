package handlers

import (
	"github.com/gin-gonic/gin"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
)

type members struct{}

func (*members) ListMembers(c *gin.Context)  { notImplemented(c) }
func (*members) CreateMember(c *gin.Context) { notImplemented(c) }
func (*members) DeleteMember(c *gin.Context, memberId oapi.MemberID) {
	notImplemented(c)
}
func (*members) GetMember(c *gin.Context, memberId oapi.MemberID) {
	notImplemented(c)
}
func (*members) UpdateMember(c *gin.Context, memberId oapi.MemberID) {
	notImplemented(c)
}
