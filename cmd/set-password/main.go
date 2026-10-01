// Command set-password creates or updates a staff member's credentials and
// permission grants so manual API flows (scripts/smoke) and local development
// can authenticate as any staff role. Permissions are stored as the JSONB
// array the auth repository scans, e.g.:
//
//	echo 'Password1!' | go run ./cmd/set-password -email admin@fitcore.local \
//		-perms branches:read,branches:create
//	go run ./cmd/set-password -email admin@fitcore.local \
//		-password-file /run/secrets/fitcore-admin -perms branches:read
//
// The password is never passed as a CLI flag (it would leak via the process
// table and shell history): it comes from -password-file (a 0600 file whose
// contents are trimmed) or, when that flag is empty, from a single stdin line
// read after a prompt.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/auth"
	"github.com/PandaX185/fitcore/internal/platform/postgres"
)

// minPasswordLength is the shortest accepted password.
const minPasswordLength = 8

func main() {
	var (
		email        = flag.String("email", "", "staff email (required)")
		passwordFile = flag.String("password-file", "", "file holding the password (0600, trimmed); when empty, read one line from stdin")
		perms        = flag.String("perms", "", "comma-separated permissions")
		reactivate   = flag.Bool("reactivate", false, "allow reactivating an inactive staff account")
	)
	flag.Parse()

	normalized, err := normalizeEmail(*email)
	if err != nil {
		fail(err.Error())
	}

	password, err := readPassword(*passwordFile)
	if err != nil {
		fail(err.Error())
	}
	if len(password) < minPasswordLength {
		fail(fmt.Sprintf("password must be at least %d characters", minPasswordLength))
	}

	permissionList, err := parsePermissions(*perms)
	if err != nil {
		fail(err.Error())
	}

	reactivated, err := run(normalized, password, permissionList, *reactivate)
	if err != nil {
		fail(err.Error())
	}
	fmt.Printf("set credentials for %s (%d permissions)\n", normalized, len(permissionList))
	fmt.Fprintf(os.Stderr, "set-password: email=%s perms=%d reactivated=%t at=%s\n",
		normalized, len(permissionList), reactivated, time.Now().UTC().Format(time.RFC3339))
}

// normalizeEmail applies exactly the login normalization (lowercase + trim)
// and rejects values that cannot be an email.
func normalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(normalized, "@") {
		return "", fmt.Errorf("set-password requires -email with a valid email address")
	}
	return normalized, nil
}

// readPassword returns the trimmed password from a 0600 file, or from a
// single stdin line (after a stderr prompt) when passwordFile is empty.
func readPassword(passwordFile string) (string, error) {
	if passwordFile != "" {
		//nolint:gosec // G304: passwordFile is an explicit operator-provided path, permission-checked below.
		raw, err := os.ReadFile(passwordFile)
		if err != nil {
			return "", fmt.Errorf("read password file: %w", err)
		}
		if fi, err := os.Stat(passwordFile); err == nil {
			if perm := fi.Mode().Perm(); perm&0077 != 0 {
				return "", fmt.Errorf("refusing to read password file with permissions %04o; run chmod 600 %s", perm, passwordFile)
			}
		}
		password := strings.TrimSpace(string(raw))
		if password == "" {
			return "", fmt.Errorf("password file %s is empty", passwordFile)
		}
		return password, nil
	}

	fmt.Fprint(os.Stderr, "password: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && len(line) == 0 {
		return "", fmt.Errorf("read password from stdin: %w", err)
	}
	password := strings.TrimSpace(line)
	if password == "" {
		return "", fmt.Errorf("no password provided on stdin")
	}
	return password, nil
}

// knownPermissions is the vocabulary the auth module defines.
var knownPermissions = []auth.Permission{
	auth.PermBranchesRead, auth.PermBranchesCreate, auth.PermBranchesUpdate,
	auth.PermMembersRead, auth.PermMembersCreate, auth.PermMembersUpdate, auth.PermMembersDelete,
	auth.PermMembershipsRead, auth.PermMembershipsCreate, auth.PermMembershipsUpdate,
	auth.PermPackagesRead, auth.PermPackagesCreate, auth.PermPackagesUpdate,
	auth.PermClassesRead, auth.PermClassesCreate, auth.PermClassesUpdate, auth.PermClassesDelete,
	auth.PermBookingsRead, auth.PermBookingsCreate, auth.PermBookingsUpdate,
	auth.PermAttendanceRead, auth.PermAttendanceCreate, auth.PermAttendanceUpdate,
	auth.PermBillingRead, auth.PermBillingCreate, auth.PermBillingUpdate,
	auth.PermStaffRead, auth.PermStaffCreate, auth.PermStaffUpdate,
	auth.PermTrainersRead, auth.PermTrainersCreate, auth.PermTrainersUpdate,
}

// parsePermissions resolves a comma-separated list, validating each token
// against the shared vocabulary so a typo fails loudly instead of silently
// granting nothing.
func parsePermissions(raw string) ([]auth.Permission, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	byName := make(map[auth.Permission]bool, len(knownPermissions))
	for _, p := range knownPermissions {
		byName[p] = true
	}

	var out []auth.Permission
	for _, part := range strings.Split(raw, ",") {
		perm := auth.Permission(strings.TrimSpace(part))
		if !byName[perm] {
			return nil, fmt.Errorf("unknown permission %q", perm)
		}
		out = append(out, perm)
	}
	return out, nil
}

func run(email, password string, perms []auth.Permission, reactivate bool) (reactivated bool, err error) {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return false, fmt.Errorf("hash password: %w", err)
	}

	db, err := postgres.Open(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		return false, fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	g := db.Gorm().WithContext(context.Background())

	var existing struct {
		ID     uuid.UUID
		Active bool
	}
	if err := g.Raw(`SELECT id, active FROM staff WHERE email = ?`, email).Scan(&existing).Error; err != nil {
		return false, fmt.Errorf("lookup staff: %w", err)
	}

	permJSON, err := encodePermissions(perms)
	if err != nil {
		return false, err
	}

	if existing.ID == uuid.Nil {
		id := uuid.New()
		err = g.Exec(
			`INSERT INTO staff (id, name, email, password_hash, permissions, active)
			 VALUES (?, ?, ?, ?, ?::jsonb, true)`,
			id, email, email, hash, permJSON,
		).Error
		if err != nil {
			return false, fmt.Errorf("insert staff: %w", err)
		}
		return false, nil
	}

	if !existing.Active && !reactivate {
		return false, fmt.Errorf("staff %s is inactive; pass -reactivate to reactivate it", email)
	}

	if err := g.Exec(
		`UPDATE staff SET password_hash = ?, permissions = ?::jsonb, active = true WHERE id = ?`,
		hash, permJSON, existing.ID,
	).Error; err != nil {
		return false, fmt.Errorf("update staff: %w", err)
	}
	return !existing.Active, nil
}

func encodePermissions(perms []auth.Permission) (string, error) {
	raw := make([]string, len(perms))
	for i, p := range perms {
		raw[i] = string(p)
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return "", fmt.Errorf("encode permissions: %w", err)
	}
	return string(b), nil
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "set-password:", msg)
	os.Exit(1)
}
