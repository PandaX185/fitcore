// Package branches models FitCore's physical gym locations.
package branches

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("branch not found")

// Branch is a physical gym location.
type Branch struct {
	ID        uuid.UUID
	Name      string
	Address   string
	Latitude  float64
	Longitude float64
	CreatedAt time.Time
	UpdatedAt time.Time
}
