package auth

import (
	"context"
	"errors"
	"log/slog"
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

// dummyPasswordHash is a fixed, valid argon2id PHC hash of a random secret
// nobody knows. Login runs VerifyPassword against it on the
// unknown-email/inactive/empty-hash paths so those rejections cost roughly
// the same as a real password check. Without it an attacker could enumerate
// valid staff emails by timing (fast reject = unknown account, slow reject =
// wrong password). The result is discarded; the caller always gets
// ErrInvalidCredentials.
//
//nolint:gosec // G101: dummy argon2id hash for timing uniformity, not a credential — the secret is unknown to everyone including operators.
const dummyPasswordHash = "$argon2id$v=19$m=65536,t=1,p=4$OSJEb4+BI2bDBwPDZGkikg$IlgXcSECuhFpakHdLE76u91vnkV6syR3Ma/oQb86sx8"

// Login authenticates a staff member and issues an access token plus a
// rotating refresh token.
func (s *Service) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	creds, err := s.staff.ByEmail(ctx, normalizeEmail(email))
	if err != nil {
		if errors.Is(err, ErrStaffNotFound) {
			// Burn the same argon2 cost as a real check; see dummyPasswordHash.
			_, _ = VerifyPassword(dummyPasswordHash, password)
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if !creds.Active || creds.PasswordHash == "" {
		// Same timing rationale as above: inactive and passwordless accounts
		// must be indistinguishable from unknown ones.
		_, _ = VerifyPassword(dummyPasswordHash, password)
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
//
// The flow is read-then-validate BEFORE consuming: the presented token is
// looked up and its owner checked (must exist and be active) before any
// rotation happens. A current-token match rotates normally; a predecessor
// match inside its grace window is a benign retry and yields ErrInvalidToken
// without touching the family; anything else (unknown hash, expired or
// revoked token, lapsed grace window) is treated as theft: the whole family
// is deleted and ErrTokenReuse is returned.
//
// A post-rotation Redis revocation failure does NOT strand the caller: the
// rotation already committed, so the fresh pair is returned anyway and the
// revocation error is only logged (the AuthGuard read path stays fail-closed,
// so the stale access token is rejected on Redis errors regardless).
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*LoginResult, error) {
	hash := HashRefreshToken(refreshToken)

	rec, err := s.refresh.FindByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, ErrInvalidToken) {
			// No row knows this hash; there is no family to kill.
			return nil, ErrTokenReuse
		}
		return nil, err
	}

	creds, err := s.staff.ByID(ctx, rec.StaffID)
	if err != nil {
		if errors.Is(err, ErrStaffNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}
	if !creds.Active {
		return nil, ErrInvalidToken
	}

	newRefresh, err := NewRefreshToken()
	if err != nil {
		return nil, err
	}
	newJTI := uuid.New()
	newExpires := time.Now().Add(s.refreshTTL)

	outcome, staffID, oldJTI, err := s.refresh.RotateToken(ctx, hash, HashRefreshToken(newRefresh), newJTI, newExpires)
	if err != nil {
		return nil, err
	}
	switch outcome {
	case RotateRotated:
		// Fall through to issuance below.
	case RotateStaleRetry:
		// Benign retry with the superseded token: the successor is already
		// issued, so just reject without killing the family.
		return nil, ErrInvalidToken
	default:
		// Theft (or a race that consumed the token between our read and the
		// rotation): end every session for the owner.
		if derr := s.refresh.RevokeFamilyByStaff(ctx, rec.StaffID); derr != nil {
			return nil, derr
		}
		return nil, ErrTokenReuse
	}

	if staffID != rec.StaffID {
		return nil, ErrInvalidToken
	}

	// The access token paired with the consumed refresh token is now stale.
	// A revocation-store failure here must not strand the client: the new
	// pair is already committed, so log and continue.
	if cerr := s.revoked.Revoke(ctx, oldJTI, s.accessTTL); cerr != nil {
		slog.Warn("refresh revocation failed; returning rotated pair anyway",
			"staff_id", staffID,
			"error", cerr,
		)
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
