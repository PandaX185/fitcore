package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service coordinates credential verification, token issuance, refresh-token
// rotation and revocation.
type Service struct {
	staff      StaffAuthRepository
	refresh    RefreshTokenRepository
	revoked    RevocationStore
	issuer     *TokenIssuer
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewService(staff StaffAuthRepository, refresh RefreshTokenRepository, revoked RevocationStore, issuer *TokenIssuer, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{
		staff:      staff,
		refresh:    refresh,
		revoked:    revoked,
		issuer:     issuer,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// Verify validates an access token and returns its principal.
func (s *Service) Verify(raw string) (Principal, error) {
	return s.issuer.Verify(raw)
}

// Login authenticates a staff member and issues an access token plus a
// rotating refresh token.
func (s *Service) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	creds, err := s.staff.ByEmail(ctx, normalizeEmail(email))
	if err != nil {
		if errors.Is(err, ErrStaffNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if !creds.Active || creds.PasswordHash == "" {
		return nil, ErrInvalidCredentials
	}
	ok, err := VerifyPassword(creds.PasswordHash, password)
	if err != nil || !ok {
		return nil, ErrInvalidCredentials
	}
	return s.issue(ctx, creds)
}

// Refresh rotates a refresh token, re-reading the staff member's current
// permission set so grant changes apply no later than the access TTL.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*LoginResult, error) {
	newRefresh, err := NewRefreshToken()
	if err != nil {
		return nil, err
	}
	newJTI := uuid.New()
	newExpires := time.Now().Add(s.refreshTTL)

	staffID, oldJTI, err := s.refresh.RotateToken(ctx, HashRefreshToken(refreshToken), HashRefreshToken(newRefresh), newJTI, newExpires)
	if err != nil {
		if errors.Is(err, ErrInvalidToken) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}

	creds, err := s.staff.ByID(ctx, staffID)
	if err != nil {
		if errors.Is(err, ErrStaffNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}
	if !creds.Active {
		return nil, ErrInvalidToken
	}

	// The access token paired with the consumed refresh token is now stale.
	if cerr := s.revoked.Revoke(ctx, oldJTI, s.accessTTL); cerr != nil {
		return nil, cerr
	}

	result, err := s.issueAccess(ctx, creds, newJTI)
	if err != nil {
		return nil, err
	}
	result.RefreshToken = newRefresh
	return result, nil
}

// Logout ends the session family: the presented access token's jti is
// blacklisted and its linked refresh token is revoked.
func (s *Service) Logout(ctx context.Context, p Principal) error {
	if err := s.revoked.Revoke(ctx, p.JTI, s.accessTTL); err != nil {
		return err
	}
	return s.refresh.RevokeByJTI(ctx, p.JTI)
}

func (s *Service) issue(ctx context.Context, creds *StaffCredentials) (*LoginResult, error) {
	jti := uuid.New()
	ref, err := NewRefreshToken()
	if err != nil {
		return nil, err
	}
	expires := time.Now().Add(s.refreshTTL)
	if err := s.refresh.CreateToken(ctx, creds.ID, HashRefreshToken(ref), jti, expires); err != nil {
		return nil, err
	}

	result, err := s.issueAccess(ctx, creds, jti)
	if err != nil {
		return nil, err
	}
	result.RefreshToken = ref
	return result, nil
}

func (s *Service) issueAccess(ctx context.Context, creds *StaffCredentials, jti uuid.UUID) (*LoginResult, error) {
	token, _, err := s.issuer.Issue(Principal{
		StaffID:     creds.ID,
		JTI:         jti,
		Permissions: creds.Permissions,
	})
	if err != nil {
		return nil, err
	}
	result := LoginResultFor(token, "", s.accessTTL)
	return &result, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
