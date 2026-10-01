package auth

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrInvalidCredentials means the email/password pair did not match an
	// active staff member, or the account has no password set.
	ErrInvalidCredentials = errors.New("invalid email or password")
	// ErrInvalidToken means the presented token is not a valid, unexpired
	// access or refresh token.
	ErrInvalidToken = errors.New("invalid or expired token")
	// ErrTokenReuse means a refresh token was presented that is neither the
	// current nor a recent (grace-window) predecessor: either it never existed
	// or its grace window lapsed. It signals possible token theft, so the
	// whole refresh-token family is revoked. Like ErrInvalidToken it maps to
	// HTTP 401; callers that need to distinguish theft from expiry must check
	// for ErrTokenReuse first with errors.Is.
	ErrTokenReuse = errors.New("refresh token reuse detected")
	// ErrForbidden means the principal is authenticated but lacks a required
	// permission.
	ErrForbidden = errors.New("insufficient permissions")
	// ErrStaffNotFound is returned by the staff repository when a record is
	// absent; the service translates it to ErrInvalidCredentials at login.
	ErrStaffNotFound = errors.New("staff member not found")
)

// Principal is the authenticated identity carried by an access token.
type Principal struct {
	StaffID     uuid.UUID
	JTI         uuid.UUID
	Permissions []Permission
}

// LoginResult is returned by both login and refresh.
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int64
}

// WithTokens builds a login result with the access lifetime in seconds.
func LoginResultFor(access, refresh string, ttl time.Duration) LoginResult {
	return LoginResult{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(ttl.Seconds()),
	}
}

// StaffCredentials is the credential-bearing subset of a staff record loaded
// by the auth repository.
type StaffCredentials struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	Permissions  []Permission
	Active       bool
}

// RotateOutcome describes what a RotateToken attempt found.
type RotateOutcome int

const (
	// RotateUnknown means the presented hash matched no current token and no
	// live predecessor: the token never existed, already expired, or its
	// grace window lapsed. The caller treats this as theft.
	RotateUnknown RotateOutcome = iota
	// RotateRotated means the presented hash was the current token and it was
	// atomically consumed in favour of its successor.
	RotateRotated
	// RotateStaleRetry means the presented hash is the immediate predecessor
	// inside its grace window: a benign client retry (the first rotation
	// response was lost). The family is left untouched.
	RotateStaleRetry
)

// RefreshTokenRecord is the read-only view of a refresh-token row returned by
// FindByHash, covering both the current token and its retained predecessor.
type RefreshTokenRecord struct {
	StaffID       uuid.UUID
	JTI           uuid.UUID
	ExpiresAt     time.Time
	Revoked       bool
	PrevHash      string
	PrevExpiresAt time.Time
}
