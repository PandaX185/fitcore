package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestTokenIssuerIssueAndVerify(t *testing.T) {
	issuer, err := NewTokenIssuer([]byte("0123456789abcdef0123456789abcdef"), 15*time.Minute)
	if err != nil {
		t.Fatalf("NewTokenIssuer: %v", err)
	}
	p := Principal{
		StaffID:     uuid.New(),
		JTI:         uuid.New(),
		Permissions: []Permission{PermBranchesRead, PermMembersCreate},
	}

	raw, exp, err := issuer.Issue(p)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	got, err := issuer.Verify(raw)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.StaffID != p.StaffID || got.JTI != p.JTI {
		t.Fatalf("Verify principal = %v/%v, want %v/%v", got.StaffID, got.JTI, p.StaffID, p.JTI)
	}
	if len(got.Permissions) != 2 || got.Permissions[0] != PermBranchesRead || got.Permissions[1] != PermMembersCreate {
		t.Fatalf("Verify perms = %v, want %v", got.Permissions, p.Permissions)
	}
	wantExp := time.Now().Add(15 * time.Minute)
	if exp.Sub(wantExp) > 30*time.Second {
		t.Fatalf("expiry = %v, want ~%v", exp, wantExp)
	}
}

func TestTokenIssuerRejectsTamperedToken(t *testing.T) {
	issuer, _ := NewTokenIssuer([]byte("0123456789abcdef0123456789abcdef"), 15*time.Minute)
	p := Principal{StaffID: uuid.New(), JTI: uuid.New(), Permissions: []Permission{PermBranchesRead}}

	raw, _, err := issuer.Issue(p)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	parts := strings.Split(raw, ".")
	parts[1] = strings.ReplaceAll(parts[1], "A", "B")
	tampered := strings.Join(parts, ".")

	if _, err := issuer.Verify(tampered); err == nil {
		t.Fatal("Verify accepted tampered token")
	}
}

func TestTokenIssuerRejectsWrongSecret(t *testing.T) {
	issuer, _ := NewTokenIssuer([]byte("0123456789abcdef0123456789abcdef"), 15*time.Minute)
	other, _ := NewTokenIssuer([]byte("fedcba9876543210fedcba9876543210"), 15*time.Minute)
	p := Principal{StaffID: uuid.New(), JTI: uuid.New()}

	raw, _, err := issuer.Issue(p)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := other.Verify(raw); err == nil {
		t.Fatal("Verify accepted token signed with a different secret")
	}
}

func TestTokenIssuerRejectsExpiredToken(t *testing.T) {
	issuer, _ := NewTokenIssuer([]byte("0123456789abcdef0123456789abcdef"), time.Nanosecond)
	p := Principal{StaffID: uuid.New(), JTI: uuid.New()}

	raw, _, err := issuer.Issue(p)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if _, err := issuer.Verify(raw); err == nil {
		t.Fatal("Verify accepted expired token")
	}
}

func TestTokenIssuerRejectsWrongIssuerAndMethod(t *testing.T) {
	issuer, _ := NewTokenIssuer([]byte("0123456789abcdef0123456789abcdef"), 15*time.Minute)
	p := Principal{StaffID: uuid.New(), JTI: uuid.New()}

	claims := tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "evil",
			Subject:   p.StaffID.String(),
			ID:        p.JTI.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}
	unsigned, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("0123456789abcdef0123456789abcdef"))
	if _, err := issuer.Verify(unsigned); err == nil {
		t.Fatal("Verify accepted token with wrong issuer")
	}

	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := issuer.Verify(none); err == nil {
		t.Fatal("Verify accepted unsigned (alg none) token")
	}
}

func TestTokenIssuerLengthRequirements(t *testing.T) {
	if _, err := NewTokenIssuer([]byte("short"), time.Minute); err == nil {
		t.Fatal("NewTokenIssuer accepted a secret shorter than 32 bytes")
	}
	if _, err := NewTokenIssuer([]byte("0123456789abcdef0123456789abcdef"), 0); err == nil {
		t.Fatal("NewTokenIssuer accepted a non-positive ttl")
	}
}

func TestRefreshTokenRoundTrip(t *testing.T) {
	raw, err := NewRefreshToken()
	if err != nil {
		t.Fatalf("NewRefreshToken: %v", err)
	}
	if len(raw) < 40 {
		t.Fatalf("token too short: %d chars", len(raw))
	}
	hash := HashRefreshToken(raw)
	if len(hash) != 64 {
		t.Fatalf("hash length = %d, want 64", len(hash))
	}
	if HashRefreshToken(raw) != hash {
		t.Fatal("HashRefreshToken not deterministic")
	}
	other, _ := NewRefreshToken()
	if hash == HashRefreshToken(other) {
		t.Fatal("two distinct refresh tokens hashed equal")
	}
}

func authPerms() []Permission {
	return append([]Permission{}, All()[:6]...)
}

func TestPermissionsRoundTrip(t *testing.T) {
	issuer, _ := NewTokenIssuer([]byte("0123456789abcdef0123456789abcdef"), 15*time.Minute)
	p := Principal{StaffID: uuid.New(), JTI: uuid.New(), Permissions: authPerms()}

	raw, _, _ := issuer.Issue(p)
	got, err := issuer.Verify(raw)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(got.Permissions) != len(p.Permissions) {
		t.Fatalf("perm count = %d, want %d", len(got.Permissions), len(p.Permissions))
	}
	for i := range p.Permissions {
		if got.Permissions[i] != p.Permissions[i] {
			t.Fatalf("perm[%d] = %q, want %q", i, got.Permissions[i], p.Permissions[i])
		}
	}
}
