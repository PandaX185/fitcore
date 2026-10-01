package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeStaffRepo struct {
	byEmail func(ctx context.Context, email string) (*StaffCredentials, error)
	byID    func(ctx context.Context, id uuid.UUID) (*StaffCredentials, error)
}

func (f fakeStaffRepo) ByEmail(ctx context.Context, email string) (*StaffCredentials, error) {
	if f.byEmail == nil {
		return nil, ErrStaffNotFound
	}
	return f.byEmail(ctx, email)
}
func (f fakeStaffRepo) ByID(ctx context.Context, id uuid.UUID) (*StaffCredentials, error) {
	if f.byID == nil {
		return nil, ErrStaffNotFound
	}
	return f.byID(ctx, id)
}

type fakeRefreshRepo struct {
	create       func(ctx context.Context, staffID uuid.UUID, tokenHash string, jti uuid.UUID, expiresAt time.Time) error
	find         func(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error)
	rotate       func(ctx context.Context, tokenHash, newTokenHash string, newJTI uuid.UUID, expiresAt time.Time) (RotateOutcome, uuid.UUID, uuid.UUID, error)
	revokeByJK   func(ctx context.Context, jti uuid.UUID) error
	revokeFamily func(ctx context.Context, staffID uuid.UUID) error
}

func (f *fakeRefreshRepo) CreateToken(ctx context.Context, staffID uuid.UUID, tokenHash string, jti uuid.UUID, expiresAt time.Time) error {
	if f.create == nil {
		return nil
	}
	return f.create(ctx, staffID, tokenHash, jti, expiresAt)
}
func (f *fakeRefreshRepo) FindByHash(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error) {
	if f.find == nil {
		return nil, ErrInvalidToken
	}
	return f.find(ctx, tokenHash)
}
func (f *fakeRefreshRepo) RotateToken(ctx context.Context, tokenHash, newTokenHash string, newJTI uuid.UUID, expiresAt time.Time) (RotateOutcome, uuid.UUID, uuid.UUID, error) {
	if f.rotate == nil {
		return RotateRotated, uuid.New(), uuid.New(), nil
	}
	return f.rotate(ctx, tokenHash, newTokenHash, newJTI, expiresAt)
}
func (f *fakeRefreshRepo) RevokeByJTI(ctx context.Context, jti uuid.UUID) error {
	if f.revokeByJK == nil {
		return nil
	}
	return f.revokeByJK(ctx, jti)
}
func (f *fakeRefreshRepo) RevokeFamilyByStaff(ctx context.Context, staffID uuid.UUID) error {
	if f.revokeFamily == nil {
		return nil
	}
	return f.revokeFamily(ctx, staffID)
}

type fakeRevocations struct {
	revoked   map[uuid.UUID]time.Duration
	check     map[uuid.UUID]bool
	checkErr  error
	revokeErr error
}

func newFakeRevocations() *fakeRevocations {
	return &fakeRevocations{revoked: map[uuid.UUID]time.Duration{}, check: map[uuid.UUID]bool{}}
}
func (f *fakeRevocations) Revoke(ctx context.Context, jti uuid.UUID, ttl time.Duration) error {
	if f.revokeErr != nil {
		return f.revokeErr
	}
	f.revoked[jti] = ttl
	return nil
}
func (f *fakeRevocations) IsRevoked(ctx context.Context, jti uuid.UUID) (bool, error) {
	if f.checkErr != nil {
		return false, f.checkErr
	}
	return f.check[jti], nil
}

func testIssuer(t *testing.T) *TokenIssuer {
	t.Helper()
	issuer, err := NewTokenIssuer([]byte("0123456789abcdef0123456789abcdef"), 15*time.Minute)
	if err != nil {
		t.Fatalf("NewTokenIssuer: %v", err)
	}
	return issuer
}

func phc(t *testing.T) string {
	t.Helper()
	h, err := HashPassword("secret123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	return h
}

func TestServiceLoginSuccess(t *testing.T) {
	id := uuid.New()
	refresh := &fakeRefreshRepo{}
	svc := NewService(
		fakeStaffRepo{byEmail: func(ctx context.Context, email string) (*StaffCredentials, error) {
			if email != "staff@fitcore.local" {
				t.Fatalf("byEmail email = %q", email)
			}
			return &StaffCredentials{ID: id, Email: email, PasswordHash: phc(t), Permissions: []Permission{PermBranchesRead}, Active: true}, nil
		}},
		refresh,
		newFakeRevocations(),
		testIssuer(t),
		15*time.Minute, 24*time.Hour,
	)

	res, err := svc.Login(context.Background(), "  Staff@fitcore.LOCAL ", "secret123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" || res.TokenType != "Bearer" {
		t.Fatalf("Login result incomplete: %+v", res)
	}
	p, err := testIssuer(t).Verify(res.AccessToken)
	if err != nil {
		t.Fatalf("verify issued access token: %v", err)
	}
	if p.StaffID != id {
		t.Fatalf("access token sub = %v, want %v", p.StaffID, id)
	}
	if len(p.Permissions) != 1 || p.Permissions[0] != PermBranchesRead {
		t.Fatalf("access token perms = %v", p.Permissions)
	}
}

func TestServiceLoginRejects(t *testing.T) {
	issuer := testIssuer(t)
	newSvc := func(staff StaffAuthRepository) *Service {
		return NewService(staff, &fakeRefreshRepo{}, newFakeRevocations(), issuer, 15*time.Minute, 24*time.Hour)
	}

	t.Run("unknown email", func(t *testing.T) {
		svc := newSvc(fakeStaffRepo{})
		if _, err := svc.Login(context.Background(), "nobody@x", "pw"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("err = %v, want ErrInvalidCredentials", err)
		}
	})
	t.Run("wrong password", func(t *testing.T) {
		svc := newSvc(fakeStaffRepo{byEmail: func(ctx context.Context, email string) (*StaffCredentials, error) {
			return &StaffCredentials{ID: uuid.New(), PasswordHash: phc(t), Active: true}, nil
		}})
		if _, err := svc.Login(context.Background(), "a@x", "nope"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("err = %v, want ErrInvalidCredentials", err)
		}
	})
	t.Run("inactive account", func(t *testing.T) {
		svc := newSvc(fakeStaffRepo{byEmail: func(ctx context.Context, email string) (*StaffCredentials, error) {
			return &StaffCredentials{ID: uuid.New(), PasswordHash: phc(t), Active: false}, nil
		}})
		if _, err := svc.Login(context.Background(), "a@x", "secret123"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("err = %v, want ErrInvalidCredentials", err)
		}
	})
	t.Run("no password set", func(t *testing.T) {
		svc := newSvc(fakeStaffRepo{byEmail: func(ctx context.Context, email string) (*StaffCredentials, error) {
			return &StaffCredentials{ID: uuid.New(), PasswordHash: "", Active: true}, nil
		}})
		if _, err := svc.Login(context.Background(), "a@x", "secret123"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("err = %v, want ErrInvalidCredentials", err)
		}
	})
}

func TestServiceRefreshRotates(t *testing.T) {
	id := uuid.New()
	oldJTI := uuid.New()
	rev := newFakeRevocations()

	svc := NewService(
		fakeStaffRepo{
			byID: func(ctx context.Context, got uuid.UUID) (*StaffCredentials, error) {
				if got != id {
					t.Fatalf("staff id = %v, want %v", got, id)
				}
				return &StaffCredentials{ID: id, PasswordHash: phc(t), Permissions: []Permission{PermStaffRead}, Active: true}, nil
			},
		},
		&fakeRefreshRepo{
			find: func(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error) {
				return &RefreshTokenRecord{StaffID: id, JTI: oldJTI, ExpiresAt: time.Now().Add(time.Hour)}, nil
			},
			rotate: func(ctx context.Context, tokenHash, newTokenHash string, newJTI uuid.UUID, expiresAt time.Time) (RotateOutcome, uuid.UUID, uuid.UUID, error) {
				return RotateRotated, id, oldJTI, nil
			},
		},
		rev,
		testIssuer(t),
		15*time.Minute, 24*time.Hour,
	)

	res, err := svc.Refresh(context.Background(), "opaque-token")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if res.RefreshToken == "" {
		t.Fatal("Refresh returned no new refresh token")
	}
	if rev.revoked[oldJTI] != 15*time.Minute {
		t.Fatalf("old jti revocation = %v, want %v", rev.revoked[oldJTI], 15*time.Minute)
	}
	p, _ := testIssuer(t).Verify(res.AccessToken)
	if len(p.Permissions) != 1 || p.Permissions[0] != PermStaffRead {
		t.Fatalf("refreshed perms = %v, want [%s]", p.Permissions, PermStaffRead)
	}
}

func TestServiceRefreshRejects(t *testing.T) {
	issuer := testIssuer(t)

	t.Run("unknown token is theft", func(t *testing.T) {
		familiesKilled := 0
		svc := NewService(
			fakeStaffRepo{},
			&fakeRefreshRepo{
				find: func(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error) {
					return nil, ErrInvalidToken
				},
				revokeFamily: func(ctx context.Context, staffID uuid.UUID) error {
					familiesKilled++
					return nil
				},
			},
			newFakeRevocations(), issuer, 15*time.Minute, 24*time.Hour,
		)
		if _, err := svc.Refresh(context.Background(), "never-existed"); !errors.Is(err, ErrTokenReuse) {
			t.Fatalf("err = %v, want ErrTokenReuse", err)
		}
		if familiesKilled != 0 {
			t.Fatalf("families killed = %d, want 0 (no family is known for a hash no row knows)", familiesKilled)
		}
	})
	t.Run("stale retry is rejected without killing the family", func(t *testing.T) {
		familiesKilled := 0
		id := uuid.New()
		svc := NewService(
			fakeStaffRepo{byID: func(ctx context.Context, got uuid.UUID) (*StaffCredentials, error) {
				return &StaffCredentials{ID: id, Active: true}, nil
			}},
			&fakeRefreshRepo{
				find: func(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error) {
					return &RefreshTokenRecord{StaffID: id, JTI: uuid.New(), ExpiresAt: time.Now().Add(time.Hour)}, nil
				},
				rotate: func(ctx context.Context, tokenHash, newTokenHash string, newJTI uuid.UUID, expiresAt time.Time) (RotateOutcome, uuid.UUID, uuid.UUID, error) {
					return RotateStaleRetry, id, uuid.New(), nil
				},
				revokeFamily: func(ctx context.Context, staffID uuid.UUID) error {
					familiesKilled++
					return nil
				},
			},
			newFakeRevocations(), issuer, 15*time.Minute, 24*time.Hour,
		)
		if _, err := svc.Refresh(context.Background(), "superseded"); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("err = %v, want ErrInvalidToken", err)
		}
		if errors.Is(func() error { _, err := svc.Refresh(context.Background(), "superseded"); return err }(), ErrTokenReuse) {
			t.Fatal("stale retry reported as theft")
		}
		if familiesKilled != 0 {
			t.Fatalf("families killed = %d, want 0 (benign retry must not kill the family)", familiesKilled)
		}
	})
	t.Run("consumed or lapsed token kills the family", func(t *testing.T) {
		var killed []uuid.UUID
		id := uuid.New()
		svc := NewService(
			fakeStaffRepo{byID: func(ctx context.Context, got uuid.UUID) (*StaffCredentials, error) {
				return &StaffCredentials{ID: id, Active: true}, nil
			}},
			&fakeRefreshRepo{
				find: func(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error) {
					return &RefreshTokenRecord{StaffID: id, JTI: uuid.New(), ExpiresAt: time.Now().Add(time.Hour)}, nil
				},
				rotate: func(ctx context.Context, tokenHash, newTokenHash string, newJTI uuid.UUID, expiresAt time.Time) (RotateOutcome, uuid.UUID, uuid.UUID, error) {
					return RotateUnknown, id, uuid.New(), nil
				},
				revokeFamily: func(ctx context.Context, staffID uuid.UUID) error {
					killed = append(killed, staffID)
					return nil
				},
			},
			newFakeRevocations(), issuer, 15*time.Minute, 24*time.Hour,
		)
		if _, err := svc.Refresh(context.Background(), "replayed"); !errors.Is(err, ErrTokenReuse) {
			t.Fatalf("err = %v, want ErrTokenReuse", err)
		}
		if len(killed) != 1 || killed[0] != id {
			t.Fatalf("families killed = %v, want [%v]", killed, id)
		}
	})
	t.Run("staff gone or deactivated", func(t *testing.T) {
		for name, staff := range map[string]StaffAuthRepository{
			"gone": fakeStaffRepo{},
			"inactive": fakeStaffRepo{byID: func(ctx context.Context, got uuid.UUID) (*StaffCredentials, error) {
				return &StaffCredentials{ID: got, Active: false}, nil
			}},
		} {
			t.Run(name, func(t *testing.T) {
				svc := NewService(
					staff,
					&fakeRefreshRepo{
						find: func(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error) {
							return &RefreshTokenRecord{StaffID: uuid.New(), JTI: uuid.New(), ExpiresAt: time.Now().Add(time.Hour)}, nil
						},
						rotate: func(ctx context.Context, tokenHash, newTokenHash string, newJTI uuid.UUID, expiresAt time.Time) (RotateOutcome, uuid.UUID, uuid.UUID, error) {
							return RotateRotated, uuid.New(), uuid.New(), nil
						},
					},
					newFakeRevocations(), issuer, 15*time.Minute, 24*time.Hour,
				)
				if _, err := svc.Refresh(context.Background(), "t"); !errors.Is(err, ErrInvalidToken) {
					t.Fatalf("err = %v, want ErrInvalidToken", err)
				}
			})
		}
	})
}

func TestServiceLogout(t *testing.T) {
	rev := newFakeRevocations()
	logged := make(map[uuid.UUID]bool)
	svc := NewService(
		fakeStaffRepo{},
		&fakeRefreshRepo{revokeByJK: func(ctx context.Context, jti uuid.UUID) error {
			logged[jti] = true
			return nil
		}},
		rev, testIssuer(t), 15*time.Minute, 24*time.Hour,
	)

	jti := uuid.New()
	if err := svc.Logout(context.Background(), Principal{StaffID: uuid.New(), JTI: jti}); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if rev.revoked[jti] != 15*time.Minute {
		t.Fatalf("access jti not revoked with access ttl")
	}
	if !logged[jti] {
		t.Fatal("refresh token not revoked by jti")
	}
}

func TestServiceVerifyTracksRevoked(t *testing.T) {
	issuer := testIssuer(t)
	svc := NewService(fakeStaffRepo{}, &fakeRefreshRepo{}, newFakeRevocations(), issuer, 15*time.Minute, 24*time.Hour)

	p := Principal{StaffID: uuid.New(), JTI: uuid.New(), Permissions: []Permission{PermBranchesRead}}
	raw, _, err := issuer.Issue(p)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	got, err := svc.Verify(raw)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.JTI != p.JTI {
		t.Fatalf("Verify jti = %v, want %v", got.JTI, p.JTI)
	}
}

// TestServiceLoginRejectsUniformly pins the dummy-hash contract: unknown
// emails, inactive accounts and passwordless accounts all fail with the same
// ErrInvalidCredentials (never leaking ErrStaffNotFound or hash errors), so
// callers cannot distinguish the cases. The argon2 cost itself cannot be
// cheaply asserted in a unit test, so this covers the observable half.
func TestServiceLoginRejectsUniformly(t *testing.T) {
	issuer := testIssuer(t)
	newSvc := func(staff StaffAuthRepository) *Service {
		return NewService(staff, &fakeRefreshRepo{}, newFakeRevocations(), issuer, 15*time.Minute, 24*time.Hour)
	}

	cases := map[string]StaffAuthRepository{
		"unknown email": fakeStaffRepo{},
		"inactive account": fakeStaffRepo{byEmail: func(ctx context.Context, email string) (*StaffCredentials, error) {
			return &StaffCredentials{ID: uuid.New(), PasswordHash: phc(t), Active: false}, nil
		}},
		"no password set": fakeStaffRepo{byEmail: func(ctx context.Context, email string) (*StaffCredentials, error) {
			return &StaffCredentials{ID: uuid.New(), PasswordHash: "", Active: true}, nil
		}},
	}
	for name, staff := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := newSvc(staff).Login(context.Background(), "a@x", "secret123")
			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("err = %v, want ErrInvalidCredentials", err)
			}
			if errors.Is(err, ErrStaffNotFound) {
				t.Fatal("staff absence leaked through Login")
			}
		})
	}
}

// memRefreshRepo is a stateful in-memory RefreshTokenRepository mirroring the
// postgres rotation semantics (current + grace-window predecessor), so the
// double-use flow can be exercised end to end at the service layer.
type memRefreshRepo struct {
	staffID uuid.UUID
	current string
	jti     uuid.UUID
	expires time.Time
	prev    string
	prevExp time.Time
	alive   bool
	killed  int
}

func (m *memRefreshRepo) CreateToken(ctx context.Context, staffID uuid.UUID, tokenHash string, jti uuid.UUID, expiresAt time.Time) error {
	m.staffID, m.current, m.jti, m.expires = staffID, tokenHash, jti, expiresAt
	m.alive = true
	return nil
}

func (m *memRefreshRepo) FindByHash(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error) {
	if !m.alive || (tokenHash != m.current && tokenHash != m.prev) {
		return nil, ErrInvalidToken
	}
	return &RefreshTokenRecord{StaffID: m.staffID, JTI: m.jti, ExpiresAt: m.expires, PrevHash: m.prev, PrevExpiresAt: m.prevExp}, nil
}

func (m *memRefreshRepo) RotateToken(ctx context.Context, tokenHash, newTokenHash string, newJTI uuid.UUID, expiresAt time.Time) (RotateOutcome, uuid.UUID, uuid.UUID, error) {
	if !m.alive {
		return RotateUnknown, uuid.Nil, uuid.Nil, nil
	}
	now := time.Now()
	if tokenHash == m.current {
		if !now.Before(m.expires) {
			return RotateUnknown, m.staffID, m.jti, nil
		}
		old := m.jti
		m.prev, m.prevExp = m.current, now.Add(5*time.Minute)
		m.current, m.jti, m.expires = newTokenHash, newJTI, expiresAt
		return RotateRotated, m.staffID, old, nil
	}
	if tokenHash == m.prev && tokenHash != "" && now.Before(m.prevExp) {
		return RotateStaleRetry, m.staffID, m.jti, nil
	}
	return RotateUnknown, m.staffID, m.jti, nil
}

func (m *memRefreshRepo) RevokeByJTI(ctx context.Context, jti uuid.UUID) error { return nil }

func (m *memRefreshRepo) RevokeFamilyByStaff(ctx context.Context, staffID uuid.UUID) error {
	m.alive = false
	m.killed++
	return nil
}

// TestServiceRefreshDoubleUse drives the exact theft-detection contract:
// reusing a just-rotated refresh token hits the predecessor grace window and
// fails with ErrInvalidToken WITHOUT killing the family (the successor still
// refreshes), while a token no row knows fails with ErrTokenReuse.
func TestServiceRefreshDoubleUse(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	mem := &memRefreshRepo{}
	svc := NewService(
		fakeStaffRepo{byID: func(ctx context.Context, got uuid.UUID) (*StaffCredentials, error) {
			return &StaffCredentials{ID: id, Active: true}, nil
		}},
		mem,
		newFakeRevocations(), testIssuer(t), 15*time.Minute, 24*time.Hour,
	)

	// Seed the session directly (Login needs a staff record with a password).
	seedRefresh, err := NewRefreshToken()
	if err != nil {
		t.Fatalf("NewRefreshToken: %v", err)
	}
	seedJTI := uuid.New()
	if err := mem.CreateToken(ctx, id, HashRefreshToken(seedRefresh), seedJTI, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rotated, err := svc.Refresh(ctx, seedRefresh)
	if err != nil {
		t.Fatalf("first Refresh: %v", err)
	}
	if rotated.RefreshToken == "" || rotated.RefreshToken == seedRefresh {
		t.Fatalf("first Refresh did not issue a fresh token: %+v", rotated)
	}

	// Second use of the same token: predecessor match inside the grace
	// window -> ErrInvalidToken, family untouched.
	if _, err := svc.Refresh(ctx, seedRefresh); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("second Refresh err = %v, want ErrInvalidToken", err)
	}
	if errors.Is(func() error { _, err := svc.Refresh(ctx, seedRefresh); return err }(), ErrTokenReuse) {
		t.Fatal("predecessor replay reported as theft")
	}
	if mem.killed != 0 {
		t.Fatalf("families killed = %d, want 0 after benign retry", mem.killed)
	}

	// The successor still refreshes: the family survived the retry.
	third, err := svc.Refresh(ctx, rotated.RefreshToken)
	if err != nil {
		t.Fatalf("successor Refresh: %v (family should have survived)", err)
	}
	if third.RefreshToken == "" {
		t.Fatal("successor Refresh returned no token")
	}

	// A token no row knows is theft: ErrTokenReuse.
	if _, err := svc.Refresh(ctx, "forged-token"); !errors.Is(err, ErrTokenReuse) {
		t.Fatalf("forged Refresh err = %v, want ErrTokenReuse", err)
	}
}
