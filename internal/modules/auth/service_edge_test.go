package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

var errSentinel = errors.New("backend exploded")

func TestServiceLoginPropagatesRepoErrors(t *testing.T) {
	issuer := testIssuer(t)

	t.Run("byEmail backend failure", func(t *testing.T) {
		svc := NewService(
			fakeStaffRepo{byEmail: func(ctx context.Context, email string) (*StaffCredentials, error) {
				return nil, errSentinel
			}},
			&fakeRefreshRepo{}, newFakeRevocations(), issuer, 15*time.Minute, 24*time.Hour,
		)
		_, err := svc.Login(context.Background(), "a@x", "pw")
		if !errors.Is(err, errSentinel) {
			t.Fatalf("err = %v, want backend error propagated", err)
		}
		if errors.Is(err, ErrInvalidCredentials) {
			t.Fatal("backend failure was masked as invalid credentials")
		}
	})

	t.Run("refresh token creation failure", func(t *testing.T) {
		svc := NewService(
			fakeStaffRepo{byEmail: func(ctx context.Context, email string) (*StaffCredentials, error) {
				return &StaffCredentials{ID: uuid.New(), PasswordHash: phc(t), Active: true}, nil
			}},
			&fakeRefreshRepo{create: func(ctx context.Context, staffID uuid.UUID, tokenHash string, jti uuid.UUID, expiresAt time.Time) error {
				return errSentinel
			}},
			newFakeRevocations(), issuer, 15*time.Minute, 24*time.Hour,
		)
		if _, err := svc.Login(context.Background(), "a@x", "secret123"); !errors.Is(err, errSentinel) {
			t.Fatalf("err = %v, want backend error propagated", err)
		}
	})
}

func TestServiceLoginNormalizesEmail(t *testing.T) {
	svc := NewService(
		fakeStaffRepo{byEmail: func(ctx context.Context, email string) (*StaffCredentials, error) {
			if email != "staff@fitcore.local" {
				t.Fatalf("byEmail email = %q, want normalized", email)
			}
			return &StaffCredentials{ID: uuid.New(), PasswordHash: phc(t), Active: true}, nil
		}},
		&fakeRefreshRepo{}, newFakeRevocations(), testIssuer(t), 15*time.Minute, 24*time.Hour,
	)
	for _, email := range []string{"  STAFF@FITCORE.LOCAL  ", "Staff@FitCore.Local", "  staff@fitcore.local", "\tstaff@fitcore.local\n"} {
		if _, err := svc.Login(context.Background(), email, "secret123"); err != nil {
			t.Fatalf("Login(%q): %v", email, err)
		}
	}
}

func TestServiceRefreshPropagatesBackendErrors(t *testing.T) {
	issuer := testIssuer(t)

	t.Run("rotate backend failure", func(t *testing.T) {
		svc := NewService(
			fakeStaffRepo{},
			&fakeRefreshRepo{rotate: func(ctx context.Context, tokenHash, newTokenHash string, newJTI uuid.UUID, expiresAt time.Time) (uuid.UUID, uuid.UUID, error) {
				return uuid.Nil, uuid.Nil, errSentinel
			}},
			newFakeRevocations(), issuer, 15*time.Minute, 24*time.Hour,
		)
		_, err := svc.Refresh(context.Background(), "t")
		if !errors.Is(err, errSentinel) {
			t.Fatalf("err = %v, want backend error propagated", err)
		}
	})

	t.Run("staff lookup backend failure", func(t *testing.T) {
		svc := NewService(
			fakeStaffRepo{byID: func(ctx context.Context, id uuid.UUID) (*StaffCredentials, error) {
				return nil, errSentinel
			}},
			&fakeRefreshRepo{rotate: func(ctx context.Context, tokenHash, newTokenHash string, newJTI uuid.UUID, expiresAt time.Time) (uuid.UUID, uuid.UUID, error) {
				return uuid.New(), uuid.New(), nil
			}},
			newFakeRevocations(), issuer, 15*time.Minute, 24*time.Hour,
		)
		if _, err := svc.Refresh(context.Background(), "t"); !errors.Is(err, errSentinel) {
			t.Fatalf("err = %v, want backend error propagated", err)
		}
	})

	t.Run("revocation backend failure", func(t *testing.T) {
		rev := newFakeRevocations()
		rev.revokeErr = errSentinel
		svc := NewService(
			fakeStaffRepo{byID: func(ctx context.Context, id uuid.UUID) (*StaffCredentials, error) {
				return &StaffCredentials{ID: id, Active: true}, nil
			}},
			&fakeRefreshRepo{rotate: func(ctx context.Context, tokenHash, newTokenHash string, newJTI uuid.UUID, expiresAt time.Time) (uuid.UUID, uuid.UUID, error) {
				return uuid.New(), uuid.New(), nil
			}},
			rev, issuer, 15*time.Minute, 24*time.Hour,
		)
		if _, err := svc.Refresh(context.Background(), "t"); !errors.Is(err, errSentinel) {
			t.Fatalf("err = %v, want backend error propagated", err)
		}
	})
}

func TestServiceLogoutPropagatesBackendErrors(t *testing.T) {
	issuer := testIssuer(t)
	jti := uuid.New()

	t.Run("revocation backend failure", func(t *testing.T) {
		rev := newFakeRevocations()
		rev.revokeErr = errSentinel
		revokedByJTI := 0
		svc := NewService(
			fakeStaffRepo{},
			&fakeRefreshRepo{revokeByJK: func(ctx context.Context, got uuid.UUID) error {
				revokedByJTI++
				return nil
			}},
			rev, issuer, 15*time.Minute, 24*time.Hour,
		)
		err := svc.Logout(context.Background(), Principal{JTI: jti})
		if !errors.Is(err, errSentinel) {
			t.Fatalf("err = %v, want backend error propagated", err)
		}
		if revokedByJTI != 0 {
			t.Fatal("RevokeByJTI ran even though Revoke failed")
		}
	})

	t.Run("revoke-by-jti backend failure", func(t *testing.T) {
		svc := NewService(
			fakeStaffRepo{},
			&fakeRefreshRepo{revokeByJK: func(ctx context.Context, got uuid.UUID) error {
				return errSentinel
			}},
			newFakeRevocations(), issuer, 15*time.Minute, 24*time.Hour,
		)
		if err := svc.Logout(context.Background(), Principal{JTI: jti}); !errors.Is(err, errSentinel) {
			t.Fatalf("err = %v, want backend error propagated", err)
		}
	})
}

func TestServiceVerifyRejectsMalformedTokens(t *testing.T) {
	issuer := testIssuer(t)
	svc := NewService(fakeStaffRepo{}, &fakeRefreshRepo{}, newFakeRevocations(), issuer, 15*time.Minute, 24*time.Hour)
	for _, raw := range []string{"", "garbage", "a.b", "x.y.z"} {
		if _, err := svc.Verify(raw); err == nil {
			t.Fatalf("Verify(%q) accepted a malformed token", raw)
		}
	}
}

func TestServiceRefreshMasksStaffNotFound(t *testing.T) {
	issuer := testIssuer(t)
	svc := NewService(
		fakeStaffRepo{byID: func(ctx context.Context, id uuid.UUID) (*StaffCredentials, error) {
			return nil, ErrStaffNotFound
		}},
		&fakeRefreshRepo{rotate: func(ctx context.Context, tokenHash, newTokenHash string, newJTI uuid.UUID, expiresAt time.Time) (uuid.UUID, uuid.UUID, error) {
			return uuid.New(), uuid.New(), nil
		}},
		newFakeRevocations(), issuer, 15*time.Minute, 24*time.Hour,
	)
	if _, err := svc.Refresh(context.Background(), "t"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	} else if errors.Is(err, ErrStaffNotFound) {
		t.Fatal("ErrStaffNotFound leaked out of Refresh")
	}
}
