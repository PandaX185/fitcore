package handlers

import (
	"github.com/gin-gonic/gin"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
)

type packages struct{}

func (*packages) ListPackages(c *gin.Context)  { notImplemented(c) }
func (*packages) CreatePackage(c *gin.Context) { notImplemented(c) }
func (*packages) GetPackage(c *gin.Context, packageId oapi.PackageID) {
	notImplemented(c)
}
func (*packages) UpdatePackage(c *gin.Context, packageId oapi.PackageID) {
	notImplemented(c)
}
