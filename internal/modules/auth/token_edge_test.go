package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const edgeSecret = "0123456789abcdef0123456789abcdef"

func edgeIssuer(t *testing.T, ttl time.Duration) *TokenIssuer {
	t.Helper()
	iss, err := NewTokenIssuer([]byte(edgeSecret), ttl)
	if err != nil {
		t.Fatalf("NewTokenIssuer: %v", err)
	}
	return iss
}

// signCrafts jwt.SigningMethodHS256 token with the shared test secret and the
// given registered claims fields, bypassing the issuer so we can forge cases
// the issuer never produces.
func signCrafts(claims tokenClaims) string {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(edgeSecret))
	if err != nil {
		panic(err)
	}
	return s
}

func newValidClaims(now time.Time) tokenClaims {
	p := Principal{StaffID: uuid.New(), JTI: uuid.New()}
	return tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tokenIssuer,
			Subject:   p.StaffID.String(),
			ID:        p.JTI.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		},
	}
}

func TestTokenIssuerSecretBoundaries(t *testing.T) {
	if _, err := NewTokenIssuer(make([]byte, 31), time.Minute); err == nil {
		t.Fatal("NewTokenIssuer accepted a 31-byte secret")
	}
	if _, err := NewTokenIssuer(make([]byte, 32), time.Minute); err != nil {
		t.Fatalf("NewTokenIssuer rejected a 32-byte secret: %v", err)
	}
	if _, err := NewTokenIssuer(make([]byte, 64), time.Minute); err != nil {
		t.Fatalf("NewTokenIssuer rejected a 64-byte secret: %v", err)
	}
	if _, err := NewTokenIssuer(make([]byte, 0), time.Minute); err == nil {
		t.Fatal("NewTokenIssuer accepted an empty secret")
	}
	if _, err := NewTokenIssuer(make([]byte, 32), 0); err == nil {
		t.Fatal("NewTokenIssuer accepted zero ttl")
	}
	if _, err := NewTokenIssuer(make([]byte, 32), -time.Minute); err == nil {
		t.Fatal("NewTokenIssuer accepted a negative ttl")
	}
}

func TestTokenIssuerVerifyRejectsGarbage(t *testing.T) {
	iss := edgeIssuer(t, 15*time.Minute)
	for _, raw := range []string{
		"",
		"garbage",
		"a.b",
		"a.b.c.d",            // too many segments
		"not.a-token-.sig",   // undecodable header
		"  .  .  ",           // blank segments
		"wordle.fortnight.y", // valid shape, bogus content
	} {
		if _, err := iss.Verify(raw); err == nil {
			t.Fatalf("Verify(%q) accepted a malformed token", raw)
		}
	}
}

func TestTokenIssuerVerifyRejectsWrongMethodAlg(t *testing.T) {
	iss := edgeIssuer(t, 15*time.Minute)
	claims := newValidClaims(time.Now())

	hs512, err := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString([]byte(edgeSecret))
	if err != nil {
		t.Fatalf("sign HS512: %v", err)
	}
	if _, err := iss.Verify(hs512); err == nil {
		t.Fatal("Verify accepted a token signed with HS512")
	}

	none, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none: %v", err)
	}
	if _, err := iss.Verify(none); err == nil {
		t.Fatal("Verify accepted an unsigned (alg=none) token")
	}
}

func TestTokenIssuerVerifyRejectsBadClaims(t *testing.T) {
	iss := edgeIssuer(t, 15*time.Minute)
	now := time.Now()

	tests := []struct {
		name   string
		claims tokenClaims
	}{
		{name: "missing exp", claims: func() tokenClaims {
			c := newValidClaims(now)
			c.ExpiresAt = nil
			return c
		}()},
		{name: "missing subject", claims: func() tokenClaims {
			c := newValidClaims(now)
			c.Subject = ""
			return c
		}()},
		{name: "missing jti", claims: func() tokenClaims {
			c := newValidClaims(now)
			c.ID = ""
			return c
		}()},
		{name: "non-uuid subject", claims: func() tokenClaims {
			c := newValidClaims(now)
			c.Subject = "not-a-uuid"
			return c
		}()},
		{name: "non-uuid jti", claims: func() tokenClaims {
			c := newValidClaims(now)
			c.ID = "not-a-uuid"
			return c
		}()},
		{name: "expired", claims: func() tokenClaims {
			c := newValidClaims(now)
			c.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Minute))
			return c
		}()},
		{name: "wrong issuer", claims: func() tokenClaims {
			c := newValidClaims(now)
			c.Issuer = "someone-else"
			return c
		}()},
		{name: "not yet valid", claims: func() tokenClaims {
			c := newValidClaims(now)
			c.NotBefore = jwt.NewNumericDate(now.Add(time.Hour))
			return c
		}()},
		{name: "issued in the future", claims: func() tokenClaims {
			c := newValidClaims(now)
			c.IssuedAt = jwt.NewNumericDate(now.Add(time.Hour))
			return c
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := iss.Verify(signCrafts(tt.claims)); err == nil {
				t.Fatalf("Verify accepted token with %s", tt.name)
			}
		})
	}
}

func TestTokenIssuerVerifyPermitsMixedPermissions(t *testing.T) {
	// Verify decodes whatever permission strings are embedded; it must not
	// reject tokens carrying permissions the vocabulary does not know (they
	// are validated at grant time, not at verify time).
	iss := edgeIssuer(t, 15*time.Minute)
	p := Principal{StaffID: uuid.New(), JTI: uuid.New(), Permissions: []Permission{PermBranchesRead, "future:thing", ""}}
	raw, _, err := iss.Issue(p)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	got, err := iss.Verify(raw)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(got.Permissions) != 3 {
		t.Fatalf("perms = %v, want 3", got.Permissions)
	}
	if got.Permissions[0] != PermBranchesRead || got.Permissions[1] != "future:thing" {
		t.Fatalf("perms = %v", got.Permissions)
	}
}

func TestTokenIssuerVerifyEmptyPermissions(t *testing.T) {
	iss := edgeIssuer(t, 15*time.Minute)
	p := Principal{StaffID: uuid.New(), JTI: uuid.New()}
	raw, _, err := iss.Issue(p)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	got, err := iss.Verify(raw)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(got.Permissions) != 0 {
		t.Fatalf("perms = %v, want empty", got.Permissions)
	}
}

func TestTokenIssuerVerifyDuplicatePermissionsPreserved(t *testing.T) {
	iss := edgeIssuer(t, 15*time.Minute)
	p := Principal{StaffID: uuid.New(), JTI: uuid.New(), Permissions: []Permission{PermBranchesRead, PermBranchesRead}}
	raw, _, _ := iss.Issue(p)
	got, err := iss.Verify(raw)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(got.Permissions) != 2 {
		t.Fatalf("perms = %v, want 2 entries preserved", got.Permissions)
	}
}

func TestTokenIssuerVerifyRejectsWrongSecret(t *testing.T) {
	iss := edgeIssuer(t, 15*time.Minute)
	claim := newValidClaims(time.Now())
	other := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)
	raw, err := other.SignedString([]byte("fedcba9876543210fedcba9876543210"))
	if err != nil {
		t.Fatalf("sign with other secret: %v", err)
	}
	if _, err := iss.Verify(raw); err == nil {
		t.Fatal("Verify accepted a token signed with a different secret")
	}
}

func TestNewRefreshTokenURLSafe(t *testing.T) {
	for i := 0; i < 20; i++ {
		raw, err := NewRefreshToken()
		if err != nil {
			t.Fatalf("NewRefreshToken: %v", err)
		}
		// base64.RawURLEncoding alphabet only; no '=' padding or '/' '+' chars.
		for _, r := range raw {
			ok := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
			if !ok {
				t.Fatalf("token contains character %q outside URL-safe alphabet: %q", r, raw)
			}
		}
		if strings.Contains(raw, "=") {
			t.Fatalf("token contains padding: %q", raw)
		}
	}
}

func TestNewRefreshTokenRandom(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		raw, err := NewRefreshToken()
		if err != nil {
			t.Fatalf("NewRefreshToken: %v", err)
		}
		if seen[raw] {
			t.Fatalf("duplicate refresh token generated: %q", raw)
		}
		seen[raw] = true
	}
}
