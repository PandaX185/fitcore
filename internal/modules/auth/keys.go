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

// Ceilings applied when verifying a stored hash. Stored hashes are only ever
// produced by HashPassword, so any value far outside our own profile indicates
// store corruption — rejecting it prevents hostile hashes from forcing an
// argon2 allocation or CPU blow-up on the login path.
const (
	maxArgonMemory  uint32 = 1 << 24 // 16M blocks, ~2 GiB worst-case allocation
	maxArgonTime    uint32 = 32
	maxArgonThreads uint8  = 64
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
	param := map[string]string{}
	for _, p := range params {
		k, v, ok := strings.Cut(p, "=")
		if !ok || v == "" {
			return false, errors.New("unrecognized argon2 parameters")
		}
		param[k] = v
	}
	mem, ok := param["m"]
	if !ok {
		return false, errors.New("missing argon2 memory parameter")
	}
	decoded, err := strconv.ParseUint(mem, 10, 32)
	if err != nil {
		return false, fmt.Errorf("parse hash memory: %w", err)
	}
	memory = uint32(decoded)
	tc, ok := param["t"]
	if !ok {
		return false, errors.New("missing argon2 time parameter")
	}
	decoded, err = strconv.ParseUint(tc, 10, 32)
	if err != nil {
		return false, fmt.Errorf("parse hash time: %w", err)
	}
	timeCost = uint32(decoded)
	par, ok := param["p"]
	if !ok {
		return false, errors.New("missing argon2 parallelism parameter")
	}
	decoded, err = strconv.ParseUint(par, 10, 8)
	if err != nil {
		return false, fmt.Errorf("parse hash threads: %w", err)
	}
	threads = uint8(decoded)

	if timeCost < 1 || timeCost > maxArgonTime {
		return false, errors.New("argon2 time parameter outside supported range")
	}
	if threads < 1 || threads > maxArgonThreads {
		return false, errors.New("argon2 parallelism outside supported range")
	}
	if memory < 8*uint32(threads) || memory > maxArgonMemory {
		return false, errors.New("argon2 memory parameter outside supported range")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}
	if len(salt) == 0 {
		return false, errors.New("password hash has an empty salt")
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
