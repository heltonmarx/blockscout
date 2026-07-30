# blockscout

A Go client for the [Blockscout](https://www.blockscout.com/) API, designed for use in tests. It covers contract creation lookup, proxy contract verification, source code retrieval, and transaction queries, with built-in retry logic and optional Redis-backed rate limiting.

## Installation

```bash
go get blockscout
```

## Quick start

```go
client := blockscout.NewBlockscoutClient(
    "https://eth.blockscout.com/api",
    "your-api-key",
)

tx, err := client.GetTransactionByHash(ctx, "0xdeadbeef...")
```

## Client

```go
func NewBlockscoutClient(rawurl string, apiKey string, opts ...Options) Client
```

| Parameter | Description |
|-----------|-------------|
| `rawurl`  | Base URL of the Blockscout instance (e.g. `https://eth.blockscout.com/api`) |
| `apiKey`  | API key sent with every request; also used as the rate-limiter key |
| `opts`    | Functional options — see [Options](#options) |

### Methods

```go
// Returns the deployer address and creation transaction for each contract.
GetContractCreator(ctx context.Context, addresses []string) ([]*ContractCreator, error)

// Returns true when the proxy contract at address is verified on-chain.
VerifyProxyContract(ctx context.Context, address string) (bool, error)

// Returns the verified source code and metadata for a contract.
// Returns ErrContractSourceNotFound when the contract is not verified.
GetContractSourceCode(ctx context.Context, address string) (*Contract, error)

// Returns transaction details for the given hash.
GetTransactionByHash(ctx context.Context, txHash string) (*Tx, error)
```

### Sentinel errors

```go
blockscout.ErrContractSourceNotFound  // contract exists but is not verified
blockscout.ErrRateLimitAchieved       // local rate limiter rejected the call
```

### Options

```go
func WithRateLimiter(limiter RateLimiter) Options
```

Attaches a rate limiter to the client. Every call to `get` checks `IsRateLimited` before issuing an HTTP request and returns `ErrRateLimitAchieved` immediately when the limit is exceeded.

## Retry

Every API call is retried up to **3 times** with exponential back-off and up to 100 ms of random jitter (capped at 1 s per attempt). A call is retried when the error is:

| Condition | Retried |
|-----------|---------|
| `context.Canceled` / `context.DeadlineExceeded` | No — propagated immediately |
| HTTP 429 Too Many Requests | **Yes** — server-side rate limit, back off and retry |
| HTTP 5xx | No — marked `Unrecoverable`, stops the loop immediately |
| Any other HTTP error (4xx, etc.) | No |
| `net.Error` (connection timeout, reset) | Yes |
| `io.EOF` / `io.ErrUnexpectedEOF` | Yes |

## Rate limiter

```go
func NewRateLimiter(ctx context.Context, redisURL string, tier Tier) (*rateLimiter, error)
```

A sliding fixed-window counter backed by Redis. The window size and request limit are set by the `Tier` passed at construction. The counter is incremented and the TTL is set atomically via a Lua script, so the key always expires correctly even if the process is interrupted between calls.

### Tiers

Predefined tiers match the [Blockscout API limits](https://docs.blockscout.com/devs/apis/requests-and-limits):

| Constant | Limit | Window | Use case |
|----------|-------|--------|----------|
| `TierNoKey` | 3 req | per second | Unauthenticated / by IP |
| `TierTemporary` | 5 req | per second | PRO free plan / temporary token |
| `TierAPIKey` | 10 req | per second | Individual API key |
| `TierWhitelisted` | 25 req | per second | Whitelisted IP |
| `TierCSVExport` | 50 req | per hour | CSV export / token holder endpoints |

### Custom tier

```go
tier := blockscout.Tier{
    Limit:  7,
    Window: blockscout.WindowSecond,
}
```

Available windows: `WindowSecond`, `WindowMinute`, `WindowHour`.

### RateLimiter interface

The client accepts any implementation of:

```go
type RateLimiter interface {
    IsRateLimited(ctx context.Context, index string) (bool, error)
}
```

This makes it straightforward to substitute a different backend (in-memory, token bucket, etc.) in tests.

## Usage examples

### With rate limiting

```go
limiter, err := blockscout.NewRateLimiter(ctx, "redis://localhost:6379", blockscout.TierAPIKey)
if err != nil {
    log.Fatal(err)
}

client := blockscout.NewBlockscoutClient(
    "https://eth.blockscout.com/api",
    "your-api-key",
    blockscout.WithRateLimiter(limiter),
)
```

### Handle rate limit exhaustion

```go
creators, err := client.GetContractCreator(ctx, addresses)
if errors.Is(err, blockscout.ErrRateLimitAchieved) {
    // local window exhausted — back off before retrying
}
```

### Contract source code

```go
contract, err := client.GetContractSourceCode(ctx, "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
if errors.Is(err, blockscout.ErrContractSourceNotFound) {
    // contract is not verified
}
fmt.Println(contract.ContractName, contract.CompilerVersion)
```

## Dependencies

| Package | Purpose |
|---------|---------|
| [`github.com/avast/retry-go/v5`](https://github.com/avast/retry-go) | Retry with back-off and jitter |
| [`github.com/redis/go-redis/v9`](https://github.com/redis/go-redis) | Redis client for the rate limiter |
