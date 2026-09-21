package handlers

import (
	"github.com/gin-gonic/gin"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
)

type branches struct{}

func (*branches) ListBranches(c *gin.Context) { notImplemented(c) }
func (*branches) CreateBranch(c *gin.Context) { notImplemented(c) }
func (*branches) GetBranch(c *gin.Context, branchId oapi.BranchID) {
	notImplemented(c)
}
func (*branches) UpdateBranch(c *gin.Context, branchId oapi.BranchID) {
	notImplemented(c)
}
