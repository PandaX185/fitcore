package handlers

import (
	"github.com/gin-gonic/gin"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
)

type staff struct{}

func (*staff) ListBranchStaff(c *gin.Context, branchId oapi.BranchID) {
	notImplemented(c)
}
func (*staff) CreateStaff(c *gin.Context) { notImplemented(c) }
func (*staff) GetStaff(c *gin.Context, staffId oapi.StaffID) {
	notImplemented(c)
}
func (*staff) UpdateStaff(c *gin.Context, staffId oapi.StaffID) {
	notImplemented(c)
}
