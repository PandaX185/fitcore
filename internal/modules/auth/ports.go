package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AuthRepository is the auth module's persistence surface. Implementations
// live in internal/platform/postgres.
type AuthRepository interface {
	StaffAuthRepository
	RefreshTokenRepository
}

// StaffAuthRepository loads the credential-bearing subset of staff records.
type StaffAuthRepository interface {
	// ByEmail returns the staff record matching email, or wraps ErrStaffNotFound.
	ByEmail(ctx context.Context, email string) (*StaffCredentials, error)
	// ByID returns the staff record by id, or wraps ErrStaffNotFound.
	ByID(ctx context.Context, id uuid.UUID) (*StaffCredentials, error)
}

// RefreshTokenRepository persists and rotates opaque refresh tokens. A token
// is one-time use: rotation atomically consumes the presented token and
// stores its successor, so concurrent replays of an old token fail.
type RefreshTokenRepository interface {
	// CreateToken inserts a fresh refresh token for the given staff member,
	// linked to the jti of the simultaneously issued access token.
	CreateToken(ctx context.Context, staffID uuid.UUID, tokenHash string, jti uuid.UUID, expiresAt time.Time) error
	// RotateToken consumes the token with the given hash and stores its
	// successor. It returns the owning staff id and the jti of the access
	// token it was linked to (so that token can be revoked). Returns a token
	// that wraps ErrInvalidToken when the presented token is unknown,
	// revoked or expired.
	RotateToken(ctx context.Context, tokenHash, newTokenHash string, newJTI uuid.UUID, expiresAt time.Time) (staffID, oldJTI uuid.UUID, err error)
	// RevokeByJTI marks every refresh token linked to the given access-token
	// jti as revoked, ending the session family.
	RevokeByJTI(ctx context.Context, jti uuid.UUID) error
}

// RevocationStore records access-token jti values that must be rejected
// before their natural expiry (logout, rotation or revocation).
type RevocationStore interface {
	Revoke(ctx context.Context, jti uuid.UUID, ttl time.Duration) error
	IsRevoked(ctx context.Context, jti uuid.UUID) (bool, error)
}
