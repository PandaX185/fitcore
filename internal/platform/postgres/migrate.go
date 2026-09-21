package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	sourcefile "github.com/golang-migrate/migrate/v4/source/file"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func MigrateUp(databaseURL, dir string) error {
	return withMigrate(databaseURL, dir, func(m *migrate.Migrate) error {
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
		return nil
	})
}

func MigrateDown(databaseURL, dir string, steps int) error {
	if steps < 1 {
		return fmt.Errorf("steps must be >= 1, got %d", steps)
	}
	return withMigrate(databaseURL, dir, func(m *migrate.Migrate) error {
		if err := m.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
		return nil
	})
}

func MigrateVersion(databaseURL, dir string) error {
	return withMigrate(databaseURL, dir, func(m *migrate.Migrate) error {
		version, dirty, err := m.Version()
		if errors.Is(err, migrate.ErrNilVersion) {
			fmt.Println("no migrations applied")
			return nil
		}
		if err != nil {
			return err
		}
		fmt.Printf("current version: %d (dirty: %t)\n", version, dirty)
		return nil
	})
}

func withMigrate(databaseURL, dir string, fn func(*migrate.Migrate) error) error {
	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open migration database: %w", err)
	}
	sqlDB.SetConnMaxLifetime(10 * time.Minute)
	defer func() { _ = sqlDB.Close() }()

	driver, err := pgxmigrate.WithInstance(sqlDB, &pgxmigrate.Config{})
	if err != nil {
		return fmt.Errorf("build migration driver: %w", err)
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve migrations directory: %w", err)
	}
	src, err := (&sourcefile.File{}).Open("file://" + filepath.ToSlash(abs))
	if err != nil {
		return fmt.Errorf("open migrations source: %w", err)
	}

	m, err := migrate.NewWithInstance("file", src, "pgx5", driver)
	if err != nil {
		return fmt.Errorf("build migrate instance: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	return fn(m)
}
