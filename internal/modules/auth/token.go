package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	tokenIssuer     = "fitcore"
	tokenMethod     = "HS256"
	refreshTokenLen = 32
)

type tokenClaims struct {
	jwt.RegisteredClaims
	Permissions []string `json:"perms"`
}

// TokenIssuer signs and verifies short-lived HMAC access tokens. Claims carry
// sub (staff id), jti (token id), perms (permission set), iat and exp.
type TokenIssuer struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenIssuer(secret []byte, ttl time.Duration) (*TokenIssuer, error) {
	if len(secret) < 32 {
		return nil, errors.New("token secret must be at least 32 bytes")
	}
	if ttl <= 0 {
		return nil, errors.New("access token ttl must be positive")
	}
	return &TokenIssuer{secret: secret, ttl: ttl}, nil
}

// Issue signs an access token for the principal and returns the token string
// and its expiry.
func (t *TokenIssuer) Issue(p Principal) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(t.ttl)
	claims := tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tokenIssuer,
			Subject:   p.StaffID.String(),
			ID:        p.JTI.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
		Permissions: permStrings(p.Permissions),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, exp, nil
}

// Verify validates the token's signature, issuer and expiry, and returns the
// embedded principal. Revocation is checked separately by the caller.
func (t *TokenIssuer) Verify(raw string) (Principal, error) {
	claims := &tokenClaims{}
	parsed, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %q", token.Method.Alg())
		}
		return t.secret, nil
	}, jwt.WithValidMethods([]string{tokenMethod}), jwt.WithIssuer(tokenIssuer), jwt.WithExpirationRequired())
	if err != nil || !parsed.Valid {
		return Principal{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	staffID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: bad subject", ErrInvalidToken)
	}
	jti, err := uuid.Parse(claims.ID)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: bad jti", ErrInvalidToken)
	}
	perms := make([]Permission, 0, len(claims.Permissions))
	for _, s := range claims.Permissions {
		perms = append(perms, Permission(s))
	}
	return Principal{StaffID: staffID, JTI: jti, Permissions: perms}, nil
}

// NewRefreshToken generates an opaque, URL-safe refresh token.
func NewRefreshToken() (string, error) {
	buf := make([]byte, refreshTokenLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashRefreshToken derives the stored form of a refresh token. The plaintext
// is never persisted.
func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
