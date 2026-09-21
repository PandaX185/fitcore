package handlers

import (
	"github.com/gin-gonic/gin"

	oapi "github.com/PandaX185/fitcore/internal/httpapi/openapi"
)

type trainers struct{}

func (*trainers) ListBranchTrainers(c *gin.Context, branchId oapi.BranchID) {
	notImplemented(c)
}
func (*trainers) CreateTrainer(c *gin.Context) { notImplemented(c) }
func (*trainers) GetTrainer(c *gin.Context, trainerId oapi.TrainerID) {
	notImplemented(c)
}
func (*trainers) UpdateTrainer(c *gin.Context, trainerId oapi.TrainerID) {
	notImplemented(c)
}
