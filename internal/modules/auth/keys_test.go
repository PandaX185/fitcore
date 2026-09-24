package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("s3cret-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=1,p=4$") {
		t.Fatalf("unexpected hash prefix: %s", hash)
	}

	ok, err := VerifyPassword(hash, "s3cret-password")
	if err != nil || !ok {
		t.Fatalf("VerifyPassword(correct) = %v, %v", ok, err)
	}
	ok, err = VerifyPassword(hash, "wrong")
	if err != nil {
		t.Fatalf("VerifyPassword(wrong): %v", err)
	}
	if ok {
		t.Fatal("VerifyPassword accepted an incorrect password")
	}
}

func TestVerifyPasswordSalts(t *testing.T) {
	a, _ := HashPassword("pw")
	b, _ := HashPassword("pw")
	if a == b {
		t.Fatal("identical hashes for the same password; salt missing")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	for _, bad := range []string{"", "not-a-hash", "$argon2id$m=bad", "$bcrypt$v=19$x$x$x$x$x"} {
		if _, err := VerifyPassword(bad, "pw"); err == nil {
			t.Fatalf("VerifyPassword(%q) did not error", bad)
		}
	}
}
