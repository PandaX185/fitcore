package postgres

import (
	"context"
	"fmt"
	"time"

	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	conn *gorm.DB
}

const (
	// DefaultMaxOpenConns caps the app-side pool. It must stay <= the
	// pgbouncer DEFAULT_POOL_SIZE (20): under POOL_MODE=session every app
	// connection can pin a server connection, so the app pool must never
	// exceed the pooler.
	DefaultMaxOpenConns = 15
	// DefaultMaxIdleConns bounds idle app-side connections.
	DefaultMaxIdleConns = 5
)

func Open(ctx context.Context, databaseURL string) (*DB, error) {
	return OpenWithPool(ctx, databaseURL, DefaultMaxOpenConns, DefaultMaxIdleConns)
}

func OpenWithPool(ctx context.Context, databaseURL string, maxOpen, maxIdle int) (*DB, error) {
	conn, err := gorm.Open(gormpostgres.New(gormpostgres.Config{
		DSN: databaseURL,
		// PreferSimpleProtocol avoids server-side prepared statements, which
		// pgbouncer cannot honor under POOL_MODE=session (see
		// deploy/docker-compose.yml). If the pool mode ever changes, revisit
		// this together with the statement cache — hence pinned explicitly.
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		// TranslateError maps PG error codes (e.g. 23505 unique violation) to
		// gorm sentinels like gorm.ErrDuplicatedKey so callers can map them.
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open gorm connection: %w", err)
	}

	sqlDB, err := conn.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{conn: conn}, nil
}

func (d *DB) Gorm() *gorm.DB {
	return d.conn
}

func (d *DB) Ping(ctx context.Context) error {
	sqlDB, err := d.conn.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func (d *DB) Close() error {
	sqlDB, err := d.conn.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
