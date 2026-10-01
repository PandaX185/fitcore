package postgres

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/PandaX185/fitcore/internal/modules/auth"
)

// AuthRepository persists staff credentials and rotating refresh tokens.
type AuthRepository struct {
	db *DB
}

func NewAuthRepository(db *DB) *AuthRepository {
	return &AuthRepository{db: db}
}

// permissionList adapts the staff.permissions JSON array to domain types.
type permissionList []auth.Permission

func (p permissionList) Value() (driver.Value, error) {
	if p == nil {
		return "[]", nil
	}
	raw := make([]string, len(p))
	for i, perm := range p {
		raw[i] = string(perm)
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (p *permissionList) Scan(src any) error {
	var raw string
	switch v := src.(type) {
	case nil:
		*p = nil
		return nil
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return fmt.Errorf("unsupported permissions scan type %T", src)
	}
	var perms []string
	if err := json.Unmarshal([]byte(raw), &perms); err != nil {
		return err
	}
	out := make(permissionList, len(perms))
	for i, s := range perms {
		out[i] = auth.Permission(s)
	}
	*p = out
	return nil
}

// staffAuthRow is the credential-bearing subset of the staff table.
type staffAuthRow struct {
	ID           uuid.UUID      `gorm:"column:id"`
	Email        string         `gorm:"column:email"`
	PasswordHash string         `gorm:"column:password_hash"`
	Permissions  permissionList `gorm:"column:permissions"`
	Active       bool           `gorm:"column:active"`
}

func (staffAuthRow) TableName() string { return "staff" }

// refreshTokenRow is the rotating refresh-token store. prev_token_hash /
// prev_expires_at retain the immediate predecessor for a short grace window
// (migration 000007) so benign client retries are distinguishable from theft.
type refreshTokenRow struct {
	ID            uuid.UUID  `gorm:"column:id"`
	StaffID       uuid.UUID  `gorm:"column:staff_id"`
	TokenHash     string     `gorm:"column:token_hash"`
	JTI           uuid.UUID  `gorm:"column:jti"`
	ExpiresAt     time.Time  `gorm:"column:expires_at"`
	Revoked       bool       `gorm:"column:revoked"`
	PrevHash      string     `gorm:"column:prev_token_hash"`
	PrevExpiresAt *time.Time `gorm:"column:prev_expires_at"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at"`
}

func (refreshTokenRow) TableName() string { return "refresh_tokens" }

// refreshPrevGracePeriod is how long a superseded refresh token stays
// recognizable as "stale but benign" after rotation. Replays inside the
// window yield ErrInvalidToken without killing the family; replays after it
// are treated as theft.
const refreshPrevGracePeriod = 5 * time.Minute

//nolint:gosec // G101: SQL column list for refresh_tokens, not a credential.
const refreshTokenColumns = `id, staff_id, token_hash, jti, expires_at, revoked,
	prev_token_hash, prev_expires_at, created_at, updated_at`

func (r *AuthRepository) ByEmail(ctx context.Context, email string) (*auth.StaffCredentials, error) {
	return r.byWhere(ctx, "email = ?", email)
}

func (r *AuthRepository) ByID(ctx context.Context, id uuid.UUID) (*auth.StaffCredentials, error) {
	return r.byWhere(ctx, "id = ?", id)
}

func (r *AuthRepository) byWhere(ctx context.Context, query string, args ...any) (*auth.StaffCredentials, error) {
	var row staffAuthRow
	err := FromContext(ctx, r.db.Gorm()).WithContext(ctx).Where(query, args...).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, auth.ErrStaffNotFound
	}
	if err != nil {
		return nil, err
	}
	return &auth.StaffCredentials{
		ID:           row.ID,
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		Permissions:  row.Permissions,
		Active:       row.Active,
	}, nil
}

func (r *AuthRepository) CreateToken(ctx context.Context, staffID uuid.UUID, tokenHash string, jti uuid.UUID, expiresAt time.Time) error {
	return FromContext(ctx, r.db.Gorm()).WithContext(ctx).Create(&refreshTokenRow{
		ID:        uuid.New(),
		StaffID:   staffID,
		TokenHash: tokenHash,
		JTI:       jti,
		ExpiresAt: expiresAt,
	}).Error
}

// FindByHash is a read-only lookup of the refresh-token row whose current or
// predecessor hash matches. It returns a token wrapping auth.ErrInvalidToken
// when no row matches. Callers validate the record (and its owner) before
// consuming anything via RotateToken.
//
// A current-hash hit on a revoked or expired row, and a predecessor-hash hit
// whose grace window lapsed, still return the row: the caller (or RotateToken)
// decides whether that is a benign retry or theft.
func (r *AuthRepository) FindByHash(ctx context.Context, tokenHash string) (*auth.RefreshTokenRecord, error) {
	var row refreshTokenRow
	res := FromContext(ctx, r.db.Gorm()).WithContext(ctx).Raw(
		`SELECT `+refreshTokenColumns+` FROM refresh_tokens
		  WHERE token_hash = ? OR prev_token_hash = ?
		  ORDER BY CASE WHEN token_hash = ? THEN 0 ELSE 1 END
		  LIMIT 1`,
		tokenHash, tokenHash, tokenHash,
	).Scan(&row)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, fmt.Errorf("find refresh token: %w", auth.ErrInvalidToken)
	}
	rec := &auth.RefreshTokenRecord{
		StaffID:   row.StaffID,
		JTI:       row.JTI,
		ExpiresAt: row.ExpiresAt,
		Revoked:   row.Revoked,
		PrevHash:  row.PrevHash,
	}
	if row.PrevExpiresAt != nil {
		rec.PrevExpiresAt = *row.PrevExpiresAt
	}
	return rec, nil
}

// RotateToken atomically consumes the presented refresh token and stores its
// successor, retaining the predecessor (prev_* = current before overwrite)
// for the grace window. The candidate rows are locked for the transaction so
// two concurrent replays cannot both succeed: the loser blocks on the lock,
// then sees the already-rotated row and lands on the stale-retry path.
//
//   - current-hash hit on a live, unrevoked row: rotate and return
//     auth.RotateRotated with the owning staff id and the jti of the access
//     token the consumed refresh token was issued alongside.
//   - predecessor-hash hit inside its grace window on a live row: no write,
//     auth.RotateStaleRetry (benign retry, family untouched).
//   - anything else (unknown hash, revoked row, expired token, lapsed grace
//     window): no write, auth.RotateUnknown. The caller treats this as theft
//     and deletes the family.
func (r *AuthRepository) RotateToken(ctx context.Context, tokenHash, newTokenHash string, newJTI uuid.UUID, expiresAt time.Time) (auth.RotateOutcome, uuid.UUID, uuid.UUID, error) {
	now := time.Now()
	var outcome auth.RotateOutcome
	var staffID, oldJTI uuid.UUID
	err := FromContext(ctx, r.db.Gorm()).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current refreshTokenRow
		res := tx.Raw(
			`SELECT `+refreshTokenColumns+` FROM refresh_tokens WHERE token_hash = ? FOR UPDATE`,
			tokenHash,
		).Scan(&current)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			if current.Revoked || !now.Before(current.ExpiresAt) {
				outcome = auth.RotateUnknown
				staffID, oldJTI = current.StaffID, current.JTI
				return nil
			}
			prevExpires := now.Add(refreshPrevGracePeriod)
			upd := tx.Exec(
				`UPDATE refresh_tokens
				    SET prev_token_hash = token_hash, prev_expires_at = ?,
				        token_hash = ?, jti = ?, expires_at = ?, updated_at = now()
				  WHERE id = ? AND token_hash = ?`,
				prevExpires, newTokenHash, newJTI, expiresAt, current.ID, tokenHash,
			)
			if upd.Error != nil {
				return upd.Error
			}
			if upd.RowsAffected == 0 {
				// Lost a race with a concurrent rotation despite the lock;
				// report unknown so the caller re-reads and decides.
				outcome = auth.RotateUnknown
				staffID, oldJTI = current.StaffID, current.JTI
				return nil
			}
			outcome = auth.RotateRotated
			staffID, oldJTI = current.StaffID, current.JTI
			return nil
		}

		var prev refreshTokenRow
		pres := tx.Raw(
			`SELECT `+refreshTokenColumns+` FROM refresh_tokens WHERE prev_token_hash = ? FOR UPDATE`,
			tokenHash,
		).Scan(&prev)
		if pres.Error != nil {
			return pres.Error
		}
		if pres.RowsAffected == 0 {
			return fmt.Errorf("rotate refresh token: %w", auth.ErrInvalidToken)
		}
		if !prev.Revoked && prev.PrevExpiresAt != nil && now.Before(*prev.PrevExpiresAt) {
			outcome = auth.RotateStaleRetry
			staffID, oldJTI = prev.StaffID, prev.JTI
			return nil
		}
		outcome = auth.RotateUnknown
		staffID, oldJTI = prev.StaffID, prev.JTI
		return nil
	})
	if err != nil {
		if errors.Is(err, auth.ErrInvalidToken) {
			return auth.RotateUnknown, uuid.Nil, uuid.Nil, nil
		}
		return auth.RotateUnknown, uuid.Nil, uuid.Nil, err
	}
	return outcome, staffID, oldJTI, nil
}

func (r *AuthRepository) RevokeByJTI(ctx context.Context, jti uuid.UUID) error {
	return FromContext(ctx, r.db.Gorm()).WithContext(ctx).
		Model(&refreshTokenRow{}).
		Where("jti = ?", jti).
		UpdateColumn("revoked", true).
		Error
}

// RevokeFamilyByStaff deletes every refresh-token row for the given staff
// member, ending all of their sessions after detected token theft.
func (r *AuthRepository) RevokeFamilyByStaff(ctx context.Context, staffID uuid.UUID) error {
	return FromContext(ctx, r.db.Gorm()).WithContext(ctx).
		Where("staff_id = ?", staffID).
		Delete(&refreshTokenRow{}).
		Error
}
