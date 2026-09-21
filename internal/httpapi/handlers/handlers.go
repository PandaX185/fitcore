package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PandaX185/fitcore/internal/modules/branches"
	"github.com/PandaX185/fitcore/internal/platform/postgres"
	"github.com/PandaX185/fitcore/internal/platform/telemetry"
)

// Handler implements the generated openapi.ServerInterface. Module adapters
// are embedded so their promoted methods cover the whole interface; each is
// filled in as its domain logic lands.
type Handler struct {
	*branchesHandler
	*members
	*memberships
	*packages
	*classes
	*bookings
	*attendance
	*billing
	*staff
	*trainers
}

func New(log *slog.Logger, metrics *telemetry.Metrics, db *postgres.DB) *Handler {
	branchSvc := branches.NewService(postgres.NewBranchRepository(db))
	return &Handler{
		branchesHandler: &branchesHandler{
			svc:     branchSvc,
			log:     log,
			metrics: metrics,
		},
		members:     &members{},
		memberships: &memberships{},
		packages:    &packages{},
		classes:     &classes{},
		bookings:    &bookings{},
		attendance:  &attendance{},
		billing:     &billing{},
		staff:       &staff{},
		trainers:    &trainers{},
	}
}

func notImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}
