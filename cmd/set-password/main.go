// Command set-password creates or updates a staff member's credentials and
// permission grants so manual API flows (scripts/smoke) and local development
// can authenticate as any staff role. Permissions are stored as the JSONB
// array the auth repository scans, e.g.:
//
//	go run ./cmd/set-password -email admin@fitcore.local \
//		-password 'Password1!' -perms branches:read,branches:create
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/modules/auth"
	"github.com/PandaX185/fitcore/internal/platform/postgres"
)

func main() {
	var (
		email    = flag.String("email", "", "staff email (required)")
		password = flag.String("password", "", "password to hash (required)")
		perms    = flag.String("perms", "", "comma-separated permissions")
	)
	flag.Parse()

	if *email == "" || *password == "" {
		fail("set-password requires -email and -password")
	}

	permissionList, err := parsePermissions(*perms)
	if err != nil {
		fail(err.Error())
	}

	if err := run(*email, *password, permissionList); err != nil {
		fail(err.Error())
	}
	fmt.Printf("set credentials for %s (%d permissions)\n", *email, len(permissionList))
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

func run(email, password string, perms []auth.Permission) error {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	db, err := postgres.Open(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	g := db.Gorm().WithContext(context.Background())

	var existing struct{ ID uuid.UUID }
	if err := g.Raw(`SELECT id FROM staff WHERE email = ?`, email).Scan(&existing).Error; err != nil {
		return fmt.Errorf("lookup staff: %w", err)
	}

	permJSON, err := encodePermissions(perms)
	if err != nil {
		return err
	}

	if existing.ID == uuid.Nil {
		id := uuid.New()
		err = g.Exec(
			`INSERT INTO staff (id, name, email, password_hash, permissions, active)
			 VALUES (?, ?, ?, ?, ?::jsonb, true)`,
			id, email, email, hash, permJSON,
		).Error
		if err != nil {
			return fmt.Errorf("insert staff: %w", err)
		}
		return nil
	}

	if err := g.Exec(
		`UPDATE staff SET password_hash = ?, permissions = ?::jsonb, active = true WHERE id = ?`,
		hash, permJSON, existing.ID,
	).Error; err != nil {
		return fmt.Errorf("update staff: %w", err)
	}
	return nil
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
