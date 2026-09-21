package handlers

import (
	"github.com/gin-gonic/gin"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
)

type classes struct{}

func (*classes) ListClasses(c *gin.Context, params oapi.ListClassesParams) {
	notImplemented(c)
}
func (*classes) CreateClass(c *gin.Context) { notImplemented(c) }
func (*classes) DeleteClass(c *gin.Context, classId oapi.ClassID) {
	notImplemented(c)
}
func (*classes) GetClass(c *gin.Context, classId oapi.ClassID) {
	notImplemented(c)
}
func (*classes) UpdateClass(c *gin.Context, classId oapi.ClassID) {
	notImplemented(c)
}
