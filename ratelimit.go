package blockscout

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Window defines the time bucket used for rate limiting.
type Window struct {
	size int64 // bucket width in seconds (used to compute the key)
	ttl  int64 // Redis key TTL in seconds (2× window to cover clock boundaries)
}

var (
	WindowSecond = Window{size: 1, ttl: 2}
	WindowMinute = Window{size: 60, ttl: 120}
	WindowHour   = Window{size: 3600, ttl: 7200}
)

// Tier pairs a request limit with its measurement window.
// Use the predefined Blockscout tiers below, or construct a custom one.
type Tier struct {
	Limit  int
	Window Window
}

// Blockscout API rate-limit tiers (https://docs.blockscout.com/devs/apis/requests-and-limits).
var (
	TierNoKey       = Tier{Limit: 3, Window: WindowSecond}  // unauthenticated / by IP
	TierTemporary   = Tier{Limit: 5, Window: WindowSecond}  // temporary token (PRO free plan)
	TierAPIKey      = Tier{Limit: 10, Window: WindowSecond} // individual API key
	TierWhitelisted = Tier{Limit: 25, Window: WindowSecond} // whitelisted IP
	TierCSVExport   = Tier{Limit: 50, Window: WindowHour}   // CSV export / token holder endpoints
)

// luaRateLimit atomically increments the counter and sets TTL on first call.
// Returns the current count after increment.
// KEYS[1] = rate key, ARGV[1] = TTL in seconds
var luaRateLimit = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
    redis.call("EXPIRE", KEYS[1], ARGV[1])
end
return current
`)

type rateLimiter struct {
	client *redis.Client
	tier   Tier
}

func NewRateLimiter(ctx context.Context, redisURL string, tier Tier) (*rateLimiter, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opt)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &rateLimiter{client: client, tier: tier}, nil
}

func (r *rateLimiter) IsRateLimited(ctx context.Context, index string) (bool, error) {
	w := r.tier.Window
	bucket := time.Now().Unix() / w.size
	key := fmt.Sprintf("rate:%s:%d", index, bucket)

	count, err := luaRateLimit.Run(ctx, r.client, []string{key}, w.ttl).Int64()
	if err != nil {
		return false, err
	}
	return count > int64(r.tier.Limit), nil
}
