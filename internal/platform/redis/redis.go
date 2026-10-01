// Package redis wraps the shared go-redis client used by platform
// infrastructure.
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

// Client is a thin wrapper holding the shared connection pool.
type Client struct {
	rdb *goredis.Client
}

// Open parses url and verifies connectivity.
func Open(ctx context.Context, url string) (*Client, error) {
	opts, err := goredis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	rdb := goredis.NewClient(opts)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return &Client{rdb: rdb}, nil
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

// FlushDB drops all keys in the selected database. It exists for the test
// helper and should never be called at runtime.
func (c *Client) FlushDB(ctx context.Context) error {
	return c.rdb.FlushDB(ctx).Err()
}

const revokeKeyPrefix = "fitcore:auth:jti:"

// RevocationStore is the Redis-backed auth.RevocationStore. Blacklisted
// access-token jti values are stored with a TTL covering the token's
// remaining lifetime. Checks fail closed: if Redis is unreachable the token
// is treated as revoked.
type RevocationStore struct {
	c *Client
}

func NewRevocationStore(c *Client) *RevocationStore {
	return &RevocationStore{c: c}
}

// redisOpTimeout bounds every revocation-store round trip so a wedged Redis
// cannot stall auth paths longer than this.
const redisOpTimeout = time.Second

// Ping verifies connectivity to Redis within a short timeout for the
// readiness probe.
func (s *RevocationStore) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, redisOpTimeout)
	defer cancel()
	if err := s.c.rdb.Ping(pingCtx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}
	return nil
}

func (s *RevocationStore) Revoke(ctx context.Context, jti uuid.UUID, ttl time.Duration) error {
	opCtx, cancel := context.WithTimeout(ctx, redisOpTimeout)
	defer cancel()
	if err := s.c.rdb.Set(opCtx, revokeKeyPrefix+jti.String(), "1", ttl).Err(); err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	return nil
}

func (s *RevocationStore) IsRevoked(ctx context.Context, jti uuid.UUID) (bool, error) {
	opCtx, cancel := context.WithTimeout(ctx, redisOpTimeout)
	defer cancel()
	n, err := s.c.rdb.Exists(opCtx, revokeKeyPrefix+jti.String()).Result()
	if err != nil {
		return true, err
	}
	return n > 0, nil
}
