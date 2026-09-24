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
	create     func(ctx context.Context, staffID uuid.UUID, tokenHash string, jti uuid.UUID, expiresAt time.Time) error
	rotate     func(ctx context.Context, tokenHash, newTokenHash string, newJTI uuid.UUID, expiresAt time.Time) (uuid.UUID, uuid.UUID, error)
	revokeByJK func(ctx context.Context, jti uuid.UUID) error
}

func (f *fakeRefreshRepo) CreateToken(ctx context.Context, staffID uuid.UUID, tokenHash string, jti uuid.UUID, expiresAt time.Time) error {
	if f.create == nil {
		return nil
	}
	return f.create(ctx, staffID, tokenHash, jti, expiresAt)
}
func (f *fakeRefreshRepo) RotateToken(ctx context.Context, tokenHash, newTokenHash string, newJTI uuid.UUID, expiresAt time.Time) (uuid.UUID, uuid.UUID, error) {
	if f.rotate == nil {
		return uuid.New(), uuid.New(), nil
	}
	return f.rotate(ctx, tokenHash, newTokenHash, newJTI, expiresAt)
}
func (f *fakeRefreshRepo) RevokeByJTI(ctx context.Context, jti uuid.UUID) error {
	if f.revokeByJK == nil {
		return nil
	}
	return f.revokeByJK(ctx, jti)
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
		&fakeRefreshRepo{rotate: func(ctx context.Context, tokenHash, newTokenHash string, newJTI uuid.UUID, expiresAt time.Time) (uuid.UUID, uuid.UUID, error) {
			return id, oldJTI, nil
		}},
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

	t.Run("rotated token rejected", func(t *testing.T) {
		svc := NewService(
			fakeStaffRepo{},
			&fakeRefreshRepo{rotate: func(ctx context.Context, tokenHash, newTokenHash string, newJTI uuid.UUID, expiresAt time.Time) (uuid.UUID, uuid.UUID, error) {
				return uuid.Nil, uuid.Nil, ErrInvalidToken
			}},
			newFakeRevocations(), issuer, 15*time.Minute, 24*time.Hour,
		)
		if _, err := svc.Refresh(context.Background(), "old"); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("err = %v, want ErrInvalidToken", err)
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
					&fakeRefreshRepo{rotate: func(ctx context.Context, tokenHash, newTokenHash string, newJTI uuid.UUID, expiresAt time.Time) (uuid.UUID, uuid.UUID, error) {
						return uuid.New(), uuid.New(), nil
					}},
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
