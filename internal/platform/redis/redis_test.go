//go:build integration

package redis_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	fitredis "github.com/PandaX185/fitcore/internal/platform/redis"
	"github.com/PandaX185/fitcore/internal/testutil"
)

func TestRevocationStoreRoundTrip(t *testing.T) {
	client := testutil.Redis(t)
	store := fitredis.NewRevocationStore(client)

	jti := uuid.New()
	ctx := context.Background()

	if revoked, err := store.IsRevoked(ctx, jti); err != nil || revoked {
		t.Fatalf("IsRevoked(fresh) = %v, %v; want false, nil", revoked, err)
	}
	if err := store.Revoke(ctx, jti, 5*time.Minute); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if revoked, err := store.IsRevoked(ctx, jti); err != nil || !revoked {
		t.Fatalf("IsRevoked(revoked) = %v, %v; want true, nil", revoked, err)
	}
}
