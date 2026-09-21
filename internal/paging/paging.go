// Package paging provides shared cursor-pagination policy and helpers. The
// default and maximum page sizes are defined here once so every paginated
// endpoint follows the same contract, and the opaque keyset cursor is generic
// over any (sort key, id) ordering.
package paging

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

const (
	// DefaultLimit is the page size used when none is requested.
	DefaultLimit = 20
	// MaxLimit caps any single page size.
	MaxLimit = 100
)

// Limit normalizes a requested page size into [1, MaxLimit], defaulting to
// DefaultLimit for zero or negative values.
func Limit(n int) int {
	if n <= 0 {
		return DefaultLimit
	}
	if n > MaxLimit {
		return MaxLimit
	}
	return n
}

// MaybeLimit applies Limit to a pointer page size, treating absence as the
// default. It exists for adapters that hand over a `nil` query parameter.
func MaybeLimit(n *int) int {
	if n == nil {
		return DefaultLimit
	}
	return Limit(*n)
}

// Cursor is an opaque keyset cursor identifying a row in any (sort key, id)
// ordering. Key carries the primary sort value as text; ID breaks ties.
type Cursor struct {
	Key string    `json:"key"`
	ID  uuid.UUID `json:"id"`
}

// Encode renders the cursor as an opaque base64url string.
func (c Cursor) Encode() string {
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

// DecodeCursor parses an opaque cursor, rejecting malformed input and cursors
// without an id. An empty string decodes to a zero cursor for the first page.
func DecodeCursor(s string) (Cursor, error) {
	if s == "" {
		return Cursor{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, err
	}
	var c Cursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return Cursor{}, err
	}
	if c.ID == uuid.Nil {
		return Cursor{}, errors.New("cursor missing id")
	}
	return c, nil
}
