package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PandaX185/fitcore/internal/platform/postgres"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")

	fs := flag.NewFlagSet("fitcore-migrate", flag.ExitOnError)
	command := fs.String("command", "", "action: up, down, version, or create")
	steps := fs.Int("steps", 1, "number of migrations to roll back (down only)")
	name := fs.String("name", "", "migration name (create only)")
	if err := fs.Parse(os.Args[1:]); err != nil {
		exit(err)
	}

	if databaseURL == "" {
		exit(fmt.Errorf("DATABASE_URL is required"))
	}

	dir, err := migrationsDir()
	if err != nil {
		exit(err)
	}

	switch *command {
	case "up":
		err = postgres.MigrateUp(databaseURL, dir)
	case "down":
		err = postgres.MigrateDown(databaseURL, dir, *steps)
	case "version":
		err = postgres.MigrateVersion(databaseURL, dir)
	case "create":
		err = createMigration(dir, *name)
	default:
		exit(fmt.Errorf("unknown command %q; use up, down, version, or create", *command))
	}
	if err != nil {
		exit(err)
	}
}

// migrationsDir finds the migrations directory whether the binary runs from
// the repo root or from cmd/migrate.
func migrationsDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	joined := filepath.Join(cwd, "migrations")
	if _, err := os.Stat(joined); err == nil {
		return joined, nil
	}
	joined = filepath.Join(cwd, "..", "..", "migrations")
	if _, err := os.Stat(joined); err == nil {
		return joined, nil
	}
	return "", fmt.Errorf("migrations directory not found relative to %s", cwd)
}

func exit(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
