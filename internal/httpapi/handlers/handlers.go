package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler implements the generated openapi.ServerInterface. Module adapters
// are embedded so their promoted methods cover the whole interface; each is
// filled in as its domain logic lands.
type Handler struct {
	*branches
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

func New() *Handler {
	return &Handler{
		branches:    &branches{},
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
