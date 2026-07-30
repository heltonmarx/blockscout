package blockscout

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRateLimiter(t *testing.T, tier Tier) (*rateLimiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rl, err := NewRateLimiter(context.Background(), "redis://"+mr.Addr(), tier)
	require.NoError(t, err)
	return rl, mr
}

func TestNewRateLimiter_InvalidURL(t *testing.T) {
	_, err := NewRateLimiter(context.Background(), "not-a-url", TierAPIKey)
	assert.Error(t, err)
}

func TestNewRateLimiter_UnreachableServer(t *testing.T) {
	_, err := NewRateLimiter(context.Background(), "redis://127.0.0.1:1", TierAPIKey)
	assert.Error(t, err)
}

func TestIsRateLimited_UnderLimit(t *testing.T) {
	rl, _ := newTestRateLimiter(t, TierAPIKey) // limit = 10 req/sec

	for i := range 10 {
		limited, err := rl.IsRateLimited(context.Background(), "test-key")
		require.NoError(t, err)
		assert.False(t, limited, "call %d should not be rate limited", i+1)
	}
}

func TestIsRateLimited_ExceedsLimit(t *testing.T) {
	rl, _ := newTestRateLimiter(t, TierAPIKey) // limit = 10

	for range 10 {
		_, err := rl.IsRateLimited(context.Background(), "test-key")
		require.NoError(t, err)
	}

	limited, err := rl.IsRateLimited(context.Background(), "test-key")
	require.NoError(t, err)
	assert.True(t, limited, "11th call should be rate limited")
}

func TestIsRateLimited_SeparateKeysAreIndependent(t *testing.T) {
	rl, _ := newTestRateLimiter(t, Tier{Limit: 2, Window: WindowSecond})

	for range 2 {
		_, _ = rl.IsRateLimited(context.Background(), "key-a")
	}
	limitedA, err := rl.IsRateLimited(context.Background(), "key-a")
	require.NoError(t, err)
	assert.True(t, limitedA, "key-a should be rate limited")

	limitedB, err := rl.IsRateLimited(context.Background(), "key-b")
	require.NoError(t, err)
	assert.False(t, limitedB, "key-b should not be rate limited")
}

func TestIsRateLimited_WindowExpiry(t *testing.T) {
	rl, mr := newTestRateLimiter(t, Tier{Limit: 1, Window: WindowSecond})

	_, _ = rl.IsRateLimited(context.Background(), "test-key")
	limited, err := rl.IsRateLimited(context.Background(), "test-key")
	require.NoError(t, err)
	assert.True(t, limited)

	// Advance miniredis clock past the TTL so the key expires.
	mr.FastForward(3 * time.Second)

	limited, err = rl.IsRateLimited(context.Background(), "test-key")
	require.NoError(t, err)
	assert.False(t, limited, "counter should have reset after window expiry")
}

func TestIsRateLimited_AtomicTTLOnFirstIncr(t *testing.T) {
	rl, mr := newTestRateLimiter(t, Tier{Limit: 5, Window: WindowSecond})

	_, err := rl.IsRateLimited(context.Background(), "test-key")
	require.NoError(t, err)

	// The Lua script sets TTL on the first INCR; the key must have a TTL now.
	keys := mr.Keys()
	require.Len(t, keys, 1)
	ttl := mr.TTL(keys[0])
	assert.Positive(t, ttl, "key should have a positive TTL after first call")
}

func TestTierPresets(t *testing.T) {
	tests := []struct {
		name  string
		tier  Tier
		limit int
	}{
		{"TierNoKey", TierNoKey, 3},
		{"TierTemporary", TierTemporary, 5},
		{"TierAPIKey", TierAPIKey, 10},
		{"TierWhitelisted", TierWhitelisted, 25},
		{"TierCSVExport", TierCSVExport, 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.limit, tt.tier.Limit)
		})
	}
}

func TestWindowPresets(t *testing.T) {
	assert.EqualValues(t, 1, WindowSecond.size)
	assert.EqualValues(t, 60, WindowMinute.size)
	assert.EqualValues(t, 3600, WindowHour.size)

	assert.EqualValues(t, 2, WindowSecond.ttl)
	assert.EqualValues(t, 120, WindowMinute.ttl)
	assert.EqualValues(t, 7200, WindowHour.ttl)
}
