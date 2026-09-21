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

func Open(ctx context.Context, databaseURL string) (*DB, error) {
	conn, err := gorm.Open(gormpostgres.New(gormpostgres.Config{
		DSN: databaseURL,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open gorm connection: %w", err)
	}

	sqlDB, err := conn.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
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
