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

// refreshTokenRow is the rotating refresh-token store.
type refreshTokenRow struct {
	ID        uuid.UUID `gorm:"column:id"`
	StaffID   uuid.UUID `gorm:"column:staff_id"`
	TokenHash string    `gorm:"column:token_hash"`
	JTI       uuid.UUID `gorm:"column:jti"`
	ExpiresAt time.Time `gorm:"column:expires_at"`
	Revoked   bool      `gorm:"column:revoked"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (refreshTokenRow) TableName() string { return "refresh_tokens" }

func (r *AuthRepository) ByEmail(ctx context.Context, email string) (*auth.StaffCredentials, error) {
	return r.byWhere(ctx, "email = ?", email)
}

func (r *AuthRepository) ByID(ctx context.Context, id uuid.UUID) (*auth.StaffCredentials, error) {
	return r.byWhere(ctx, "id = ?", id)
}

func (r *AuthRepository) byWhere(ctx context.Context, query string, args ...any) (*auth.StaffCredentials, error) {
	var row staffAuthRow
	err := r.db.Gorm().WithContext(ctx).Where(query, args...).First(&row).Error
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
	return r.db.Gorm().WithContext(ctx).Create(&refreshTokenRow{
		ID:        uuid.New(),
		StaffID:   staffID,
		TokenHash: tokenHash,
		JTI:       jti,
		ExpiresAt: expiresAt,
	}).Error
}

// RotateToken atomically consumes the presented refresh token and stores its
// successor. The row is locked for the transaction so two concurrent replays
// cannot both succeed: the loser finds no row (already rotated) or a revoked
// flag. Returns the owning staff id and the jti of the access token the
// consumed refresh token was issued alongside.
func (r *AuthRepository) RotateToken(ctx context.Context, tokenHash, newTokenHash string, newJTI uuid.UUID, expiresAt time.Time) (uuid.UUID, uuid.UUID, error) {
	var staffID, oldJTI uuid.UUID
	err := r.db.Gorm().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row refreshTokenRow
		err := tx.Raw(
			`SELECT id, staff_id, jti, expires_at, revoked FROM refresh_tokens WHERE token_hash = ? FOR UPDATE`,
			tokenHash,
		).Scan(&row).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return auth.ErrInvalidToken
			}
			return err
		}
		if row.Revoked || time.Now().After(row.ExpiresAt) {
			return auth.ErrInvalidToken
		}

		res := tx.Exec(
			`UPDATE refresh_tokens SET token_hash = ?, jti = ?, expires_at = ?, updated_at = now() WHERE id = ?`,
			newTokenHash, newJTI, expiresAt, row.ID,
		)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errors.New("refresh token row vanished during rotation")
		}

		staffID, oldJTI = row.StaffID, row.JTI
		return nil
	})
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return staffID, oldJTI, nil
}

func (r *AuthRepository) RevokeByJTI(ctx context.Context, jti uuid.UUID) error {
	return r.db.Gorm().WithContext(ctx).
		Model(&refreshTokenRow{}).
		Where("jti = ?", jti).
		UpdateColumn("revoked", true).
		Error
}
