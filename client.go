package blockscout

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/avast/retry-go/v5"
)

var (
	ErrContractSourceNotFound = errors.New("contract source code not found")
	ErrRateLimitAchieved      = errors.New("API requests limit achieved")
)

type RateLimiter interface {
	IsRateLimited(ctx context.Context, index string) (bool, error)
}

type Client interface {
	GetContractCreator(ctx context.Context, addresses []string) ([]*ContractCreator, error)
	VerifyProxyContract(ctx context.Context, address string) (bool, error)
	GetContractSourceCode(ctx context.Context, address string) (*Contract, error)
	GetTransactionByHash(ctx context.Context, txHash string) (*Tx, error)
}

type Options func(c *client)

func WithRateLimiter(limiter RateLimiter) Options {
	return func(c *client) {
		c.limiter = limiter
	}
}

type client struct {
	client  *HTTPClient
	apiKey  string
	limiter RateLimiter
}

func NewBlockscoutClient(rawurl string, apiKey string, opts ...Options) Client {
	c := &client{
		client: NewHTTPClient(rawurl, 3*time.Second),
		apiKey: apiKey,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// GetContractCreator returns contract creator and transaction hash.
func (c *client) GetContractCreator(ctx context.Context, addresses []string) ([]*ContractCreator, error) {
	path := url.Values{}
	path.Add("module", "contract")
	path.Add("action", "getcontractcreation")
	path.Add("contractaddresses", strings.Join(addresses, ","))

	contractCreator := make([]*ContractCreator, 0)
	err := c.get(ctx, path.Encode(), &contractCreator)
	if err != nil {
		err = fmt.Errorf("failed get contract creator: %w", err)
		return nil, err
	}
	return contractCreator, nil
}

// VerifyProxyContract returns true for a verified proxy contract.
func (c *client) VerifyProxyContract(ctx context.Context, address string) (bool, error) {
	path := url.Values{}
	path.Add("module", "contract")
	path.Add("action", "verifyproxycontract")
	path.Add("address", address)

	var result string
	err := c.get(ctx, path.Encode(), &result)
	if err != nil {
		return false, fmt.Errorf("failed verify proxy contract: %w", err)
	}
	normalized := strings.ToLower(strings.TrimPrefix(address, "0x"))
	return strings.Contains(strings.ToLower(result), normalized), nil
}

// GetContractSourceCode returns contract source code for a verified contract.
func (c *client) GetContractSourceCode(ctx context.Context, address string) (*Contract, error) {
	path := url.Values{}
	path.Add("module", "contract")
	path.Add("action", "getsourcecode")
	path.Add("address", address)

	contracts := make([]*Contract, 0)
	err := c.get(ctx, path.Encode(), &contracts)
	switch {
	case err != nil:
		return nil, fmt.Errorf("failed get contract source code: %w", err)
	case len(contracts) == 0:
		return nil, ErrContractSourceNotFound
	}
	return contracts[0], nil
}

// GetTransactionByHash returns the information related to a specified transaction.
func (c *client) GetTransactionByHash(ctx context.Context, txHash string) (*Tx, error) {
	path := url.Values{}
	path.Add("module", "transaction")
	path.Add("action", "gettxinfo")
	path.Add("txhash", txHash)

	var tx Tx
	err := c.get(ctx, path.Encode(), &tx)
	if err != nil {
		err = fmt.Errorf("failed get transaction by txHash: %w", err)
		return nil, err
	}
	return &tx, nil
}

func (c *client) get(ctx context.Context, path string, v any) error {
	return c.rateLimiterMiddleware(ctx, func(ctx context.Context) error {
		opts := []retry.Option{
			retry.Attempts(3),
			retry.MaxDelay(time.Second),
			retry.MaxJitter(time.Millisecond * 100),
			retry.DelayType(retry.CombineDelay(retry.BackOffDelay, retry.RandomDelay)),
			retry.RetryIf(isRetryable),
			retry.Context(ctx),
		}
		buf, err := c.client.Get(ctx, path, opts...)
		if err != nil {
			return err
		}
		envelope := struct {
			Message string          `json:"message"`
			Status  string          `json:"status"`
			Result  json.RawMessage `json:"result"`
		}{}
		if err := json.Unmarshal(buf, &envelope); err != nil {
			return fmt.Errorf("invalid response: %w", err)
		}
		if envelope.Status != "1" {
			return fmt.Errorf("API error: %s", envelope.Message)
		}
		return json.Unmarshal(envelope.Result, v)
	})
}

func isRetryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if se, ok := errors.AsType[*statusError](err); ok {
		return !isUnrecoverable(se.code)
	}
	if _, ok := errors.AsType[net.Error](err); ok {
		return true
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return true
	}
	return false
}

func (c *client) rateLimiterMiddleware(
	ctx context.Context,
	fn func(ctx context.Context) error,
) error {
	if c.limiter != nil {
		limited, err := c.limiter.IsRateLimited(ctx, c.apiKey)
		switch {
		case err != nil:
			return err
		case limited:
			return ErrRateLimitAchieved
		}
	}
	return fn(ctx)
}
