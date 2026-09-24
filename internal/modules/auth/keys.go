package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters. m=64 MiB, t=1, p=4 is a sane OWASP-adjacent default
// for interactive credentials on modest hardware.
const (
	argonMemory  uint32 = 64 * 1024
	argonTime    uint32 = 1
	argonThreads uint8  = 4
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16
)

// HashPassword derives an argon2id PHC string for the given password, e.g.
// $argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>. The salt is random per call.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return "$argon2id$" +
		"v=" + strconv.Itoa(argon2.Version) +
		"$m=" + strconv.FormatUint(uint64(argonMemory), 10) +
		",t=" + strconv.FormatUint(uint64(argonTime), 10) +
		",p=" + strconv.FormatUint(uint64(argonThreads), 10) +
		"$" + base64.RawStdEncoding.EncodeToString(salt) +
		"$" + base64.RawStdEncoding.EncodeToString(hash), nil
}

// VerifyPassword checks a plaintext password against a PHC-encoded argon2id
// hash. A malformed hash yields an error rather than a mismatch to surface
// store corruption.
func VerifyPassword(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || !strings.HasPrefix(parts[2], "v=") {
		return false, errors.New("unrecognized password hash format")
	}

	version, err := strconv.Atoi(strings.TrimPrefix(parts[2], "v="))
	if err != nil {
		return false, fmt.Errorf("parse hash version: %w", err)
	}
	if version != argon2.Version {
		return false, fmt.Errorf("unsupported argon2 version %d", version)
	}

	var memory, timeCost uint32
	var threads uint8
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return false, errors.New("unrecognized argon2 parameters")
	}
	if _, err := fmt.Sscanf(params[0], "m=%d", &memory); err != nil {
		return false, fmt.Errorf("parse hash memory: %w", err)
	}
	if _, err := fmt.Sscanf(params[1], "t=%d", &timeCost); err != nil {
		return false, fmt.Errorf("parse hash time: %w", err)
	}
	if _, err := fmt.Sscanf(params[2], "p=%d", &threads); err != nil {
		return false, fmt.Errorf("parse hash threads: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode hash: %w", err)
	}
	if len(want) == 0 {
		return false, errors.New("password hash has invalid key length")
	}
	wantLen := len(want)
	if wantLen > math.MaxUint32 {
		return false, errors.New("password hash exceeds maximum key length")
	}

	got := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(wantLen))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
