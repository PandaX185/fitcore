//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/auth"
	"github.com/PandaX185/fitcore/internal/platform/postgres"
	"github.com/PandaX185/fitcore/internal/testutil"
)

// TestAuthRefreshRotationOutcomes pins the RotateToken outcome contract
// against a live database: current-match rotates while retaining the
// predecessor, predecessor-match inside the grace window reports stale-retry
// without writing, and unknown or grace-lapsed tokens report unknown so the
// caller can treat them as theft.
func TestAuthRefreshRotationOutcomes(t *testing.T) {
	db := testutil.DB(t)
	repo := postgres.NewAuthRepository(db)
	ctx := context.Background()

	staffID := uuid.New()
	email := "refresh-" + staffID.String() + "@fitcore.local"
	if err := db.Gorm().WithContext(ctx).Exec(
		`INSERT INTO staff (id, name, email) VALUES (?, ?, ?)`,
		staffID, "Refresh Probe", email,
	).Error; err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Gorm().WithContext(ctx).Exec(`DELETE FROM staff WHERE id = ?`, staffID).Error
	})

	h1 := auth.HashRefreshToken("refresh-one")
	jti1 := uuid.New()
	if err := repo.CreateToken(ctx, staffID, h1, jti1, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	// FindByHash is the read-only pre-validation lookup.
	rec, err := repo.FindByHash(ctx, h1)
	if err != nil {
		t.Fatalf("FindByHash: %v", err)
	}
	if rec.StaffID != staffID || rec.JTI != jti1 {
		t.Fatalf("FindByHash = %+v, want staff %v jti %v", rec, staffID, jti1)
	}
	if _, err := repo.FindByHash(ctx, auth.HashRefreshToken("nope")); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("FindByHash unknown err = %v, want ErrInvalidToken", err)
	}

	// Current-match rotates and reports the consumed access-token jti.
	h2 := auth.HashRefreshToken("refresh-two")
	jti2 := uuid.New()
	outcome, gotStaff, oldJTI, err := repo.RotateToken(ctx, h1, h2, jti2, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("RotateToken: %v", err)
	}
	if outcome != auth.RotateRotated {
		t.Fatalf("outcome = %v, want RotateRotated", outcome)
	}
	if gotStaff != staffID || oldJTI != jti1 {
		t.Fatalf("rotated staff/jti = %v/%v, want %v/%v", gotStaff, oldJTI, staffID, jti1)
	}

	// The predecessor is retained: the old hash still resolves, and replaying
	// it inside the grace window is a benign stale retry (no write, family
	// intact — the successor still looks up fine).
	if _, err := repo.FindByHash(ctx, h1); err != nil {
		t.Fatalf("FindByHash predecessor: %v", err)
	}
	outcome, _, _, err = repo.RotateToken(ctx, h1, auth.HashRefreshToken("refresh-three"), uuid.New(), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("RotateToken predecessor: %v", err)
	}
	if outcome != auth.RotateStaleRetry {
		t.Fatalf("predecessor outcome = %v, want RotateStaleRetry", outcome)
	}
	if _, err := repo.FindByHash(ctx, h2); err != nil {
		t.Fatalf("successor after stale retry: %v (family must survive)", err)
	}

	// A hash no row knows reports unknown without an error.
	outcome, _, _, err = repo.RotateToken(ctx, auth.HashRefreshToken("forged"), auth.HashRefreshToken("x"), uuid.New(), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("RotateToken unknown: %v", err)
	}
	if outcome != auth.RotateUnknown {
		t.Fatalf("unknown outcome = %v, want RotateUnknown", outcome)
	}

	// A lapsed grace window turns the predecessor replay into unknown (theft).
	if err := db.Gorm().WithContext(ctx).Exec(
		`UPDATE refresh_tokens SET prev_expires_at = now() - interval '1 minute' WHERE staff_id = ?`,
		staffID,
	).Error; err != nil {
		t.Fatalf("lapse predecessor: %v", err)
	}
	outcome, _, _, err = repo.RotateToken(ctx, h1, auth.HashRefreshToken("refresh-four"), uuid.New(), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("RotateToken lapsed: %v", err)
	}
	if outcome != auth.RotateUnknown {
		t.Fatalf("lapsed outcome = %v, want RotateUnknown", outcome)
	}

	// Family revocation deletes every row for the staff member.
	if err := repo.RevokeFamilyByStaff(ctx, staffID); err != nil {
		t.Fatalf("RevokeFamilyByStaff: %v", err)
	}
	if _, err := repo.FindByHash(ctx, h2); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("FindByHash after family kill err = %v, want ErrInvalidToken", err)
	}
}
