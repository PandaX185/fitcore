package handlers

import (
	"github.com/gin-gonic/gin"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
)

type memberships struct{}

func (*memberships) CreateMembership(c *gin.Context) { notImplemented(c) }
func (*memberships) GetMembership(c *gin.Context, membershipId oapi.MembershipID) {
	notImplemented(c)
}
func (*memberships) UpdateMembership(c *gin.Context, membershipId oapi.MembershipID) {
	notImplemented(c)
}
func (*memberships) ListMemberMemberships(c *gin.Context, memberId oapi.MemberID) {
	notImplemented(c)
}
