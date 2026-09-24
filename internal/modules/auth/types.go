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
