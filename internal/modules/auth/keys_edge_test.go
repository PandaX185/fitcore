package auth

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// validHash returns a freshly derived PHC hash for "pw", so callers can
// corrupt specific segments of a known-good structure.
func validHash(t *testing.T) string {
	t.Helper()
	h, err := HashPassword("pw")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	return h
}

func TestVerifyPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := VerifyPassword(h, "correct horse battery staple")
	if err != nil || !ok {
		t.Fatalf("VerifyPassword = %v, %v", ok, err)
	}
}

func TestVerifyPasswordEmptyAndUnicode(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{name: "empty", password: ""},
		{name: "unicode", password: string([]rune{'p', 'ä', 's', 's', 'w', 'ö', 'r', 'd', '🔒'})},
		{name: "long", password: strings.Repeat("a", 4096)},
		{name: "whitespace", password: "  "},
		{name: "newline", password: "line1\nline2"},
		{name: "nul byte", password: "a\x00b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := HashPassword(tt.password)
			if err != nil {
				t.Fatalf("HashPassword: %v", err)
			}
			ok, err := VerifyPassword(h, tt.password)
			if err != nil || !ok {
				t.Fatalf("VerifyPassword = %v, %v", ok, err)
			}
			if ok2, _ := VerifyPassword(h, tt.password+"x"); ok2 {
				t.Fatal("VerifyPassword accepted a mutated password")
			}
		})
	}
}

func TestVerifyPasswordRejectsStructuralCorruption(t *testing.T) {
	good := validHash(t)
	parts := strings.Split(good, "$")
	if len(parts) != 6 {
		t.Fatalf("unexpected split: %d parts", len(parts))
	}
	salt := parts[4]
	hash := parts[5]

	tests := []struct {
		name string
		hash string
	}{
		{name: "empty string", hash: ""},
		{name: "not a hash", hash: "plaintext"},
		{name: "wrong algorithm", hash: "$bcrypt$v=19$m=65536,t=1,p=4$" + salt + "$" + hash},
		{name: "algorithm case", hash: "$Argon2id$v=19$m=65536,t=1,p=4$" + salt + "$" + hash},
		{name: "no version", hash: "$argon2id$m=65536,t=1,p=4$" + salt + "$" + hash},
		{name: "bad version", hash: "$argon2id$v=abc$m=65536,t=1,p=4$" + salt + "$" + hash},
		{name: "unsupported version", hash: "$argon2id$v=1$m=65536,t=1,p=4$" + salt + "$" + hash},
		{name: "negative version", hash: "$argon2id$v=-2$m=65536,t=1,p=4$" + salt + "$" + hash},
		{name: "missing params", hash: "$argon2id$v=19$$" + salt + "$" + hash},
		{name: "no params", hash: "$argon2id$v=19$" + salt + "$" + hash},
		{name: "two params", hash: "$argon2id$v=19$m=65536,t=1$" + salt + "$" + hash},
		{name: "bad m", hash: "$argon2id$v=19$m=abc,t=1,p=4$" + salt + "$" + hash},
		{name: "bad t", hash: "$argon2id$v=19$m=65536,t=abc,p=4$" + salt + "$" + hash},
		{name: "bad p", hash: "$argon2id$v=19$m=65536,t=1,p=abc$" + salt + "$" + hash},
		{name: "missing colon m", hash: "$argon2id$v=19$65536,t=1,p=4$" + salt + "$" + hash},
		{name: "empty m", hash: "$argon2id$v=19$m=,t=1,p=4$" + salt + "$" + hash},
		{name: "zero m", hash: "$argon2id$v=19$m=0,t=1,p=4$" + salt + "$" + hash},
		{name: "zero t", hash: "$argon2id$v=19$m=65536,t=0,p=4$" + salt + "$" + hash},
		{name: "zero p", hash: "$argon2id$v=19$m=65536,t=1,p=0$" + salt + "$" + hash},
		{name: "bad salt base64", hash: "$argon2id$v=19$m=65536,t=1,p=4$!!!bad!!!$" + hash},
		{name: "bad hash base64", hash: "$argon2id$v=19$m=65536,t=1,p=4$" + salt + "$!!!bad!!!"},
		{name: "empty salt", hash: "$argon2id$v=19$m=65536,t=1,p=4$$" + hash},
		{name: "empty hash", hash: "$argon2id$v=19$m=65536,t=1,p=4$" + salt + "$"},
		{name: "extra segment", hash: "$argon2id$v=19$m=65536,t=1,p=4$" + salt + "$" + hash + "$x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := VerifyPassword(tt.hash, "pw"); err == nil {
				t.Fatalf("VerifyPassword(%q) did not error", tt.hash)
			}
		})
	}
}

func TestVerifyPasswordRejectsGarbageParamsJoin(t *testing.T) {
	// Malformed "m=TINY,t=1,p=4" where Sscanf partially parses — verify it
	// returns false, not a panic (safety under hostile input).
	for _, h := range []string{
		"$argon2id$v=19$m=1small,t=1,p=4$AAAAAAAAAAAAAAAAAAAAAA$BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
		"$argon2id$v=19$m=65536,t=1small,p=4$AAAAAAAAAAAAAAAAAAAAAA$BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
	} {
		ok, err := VerifyPassword(h, "pw")
		if err == nil && ok {
			t.Fatalf("VerifyPassword(%q) succeeded on garbage params", h)
		}
	}
}

func TestVerifyPasswordRejectsExcessiveMemory(t *testing.T) {
	// A hostile hash must not be able to drive a terabyte-scale argon2
	// allocation: values above the ceiling are rejected without hashing.
	good := validHash(t)
	parts := strings.Split(good, "$")
	salt, hash := parts[4], parts[5]
	base := "$argon2id$v=19$%s$" + salt + "$" + hash

	for name, param := range map[string]string{
		"one over ceiling":       "m=" + strconv.FormatUint(uint64(maxArgonMemory)+1, 10),
		"max uint32":             "m=4294967295",
		"ten million":            "m=10485760",
		"below spec minimum":     "m=7",
		"time blowup":            "m=65536,t=4294967295,p=4",
		"time above ceiling":     "m=65536,t=33,p=4",
		"parallelism overflow":   "m=65536,t=1,p=256",
		"high parallelism":       "m=65536,t=1,p=128",
		"memory below 8*threads": "m=64,t=1,p=32",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := VerifyPassword(fmt.Sprintf(base, param), "pw"); err == nil {
				t.Fatalf("VerifyPassword accepted params %q", param)
			}
		})
	}
}

func TestVerifyPasswordTinyKeyLength(t *testing.T) {
	// The decoded key length must be non-zero; a 1-byte key is legal.
	salt := base64.RawStdEncoding.EncodeToString(make([]byte, 16))
	one := base64.RawStdEncoding.EncodeToString([]byte{0x42})
	h := "$argon2id$v=19$m=65536,t=1,p=4$" + salt + "$" + one
	ok, err := VerifyPassword(h, "pw")
	if err != nil {
		t.Fatalf("VerifyPassword(1-byte key) errored: %v", err)
	}
	if ok {
		t.Fatal("VerifyPassword(1-byte key) matched, want mismatch")
	}

	empty := "$argon2id$v=19$m=65536,t=1,p=4$" + salt + "$"
	if _, err := VerifyPassword(empty, "pw"); err == nil {
		t.Fatal("VerifyPassword accepted a zero-length key")
	}
}
