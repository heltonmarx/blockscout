package blockscout

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// apiResponse wraps a value in the Blockscout envelope.
func apiResponse(t *testing.T, v any) []byte {
	t.Helper()
	result, err := json.Marshal(v)
	require.NoError(t, err)
	env, err := json.Marshal(map[string]any{
		"status":  "1",
		"message": "OK",
		"result":  json.RawMessage(result),
	})
	require.NoError(t, err)
	return env
}

// apiErrorResponse returns a Blockscout error envelope.
func apiErrorResponse(message string) []byte {
	env, _ := json.Marshal(map[string]any{
		"status":  "0",
		"message": message,
		"result":  nil,
	})
	return env
}

func newTestClient(t *testing.T, handler http.HandlerFunc, opts ...Options) Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewBlockscoutClient(srv.URL, "test-api-key", opts...)
}

// --- GetContractCreator ---

func TestGetContractCreator_Success(t *testing.T) {
	expected := []*ContractCreator{
		{ContractAddress: "0xABC", ContractCreator: "0xDEF", TxHash: "0x123"},
	}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "contract", r.URL.Query().Get("module"))
		assert.Equal(t, "getcontractcreation", r.URL.Query().Get("action"))
		assert.Equal(t, "0xABC", r.URL.Query().Get("contractaddresses"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(apiResponse(t, expected))
	})

	actual, err := c.GetContractCreator(context.Background(), []string{"0xABC"})
	require.NoError(t, err)
	require.Len(t, actual, 1)
	assert.Equal(t, expected[0].ContractAddress, actual[0].ContractAddress)
	assert.Equal(t, expected[0].ContractCreator, actual[0].ContractCreator)
	assert.Equal(t, expected[0].TxHash, actual[0].TxHash)
}

func TestGetContractCreator_MultipleAddresses(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "0xAAA,0xBBB", r.URL.Query().Get("contractaddresses"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(apiResponse(t, []*ContractCreator{}))
	})

	_, err := c.GetContractCreator(context.Background(), []string{"0xAAA", "0xBBB"})
	require.NoError(t, err)
}

func TestGetContractCreator_APIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(apiErrorResponse("Invalid address"))
	})

	_, err := c.GetContractCreator(context.Background(), []string{"0xBAD"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid address")
}

// --- VerifyProxyContract ---

func TestVerifyProxyContract_Verified(t *testing.T) {
	address := "0xAbCdEf1234567890AbCdEf1234567890AbCdEf12"
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "verifyproxycontract", r.URL.Query().Get("action"))
		w.WriteHeader(http.StatusOK)
		// Result contains the address (as returned by the API).
		_, _ = w.Write(apiResponse(t, "Contract at abcdef1234567890abcdef1234567890abcdef12 is verified"))
	})

	ok, err := c.VerifyProxyContract(context.Background(), address)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestVerifyProxyContract_NotVerified(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(apiResponse(t, "Contract is not verified"))
	})

	ok, err := c.VerifyProxyContract(context.Background(), "0x1111111111111111111111111111111111111111")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestVerifyProxyContract_CaseInsensitive(t *testing.T) {
	address := "0xABCDEF"
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// API returns the address in lowercase.
		_, _ = w.Write(apiResponse(t, "verified: abcdef"))
	})

	ok, err := c.VerifyProxyContract(context.Background(), address)
	require.NoError(t, err)
	assert.True(t, ok)
}

// --- GetContractSourceCode ---

func TestGetContractSourceCode_Success(t *testing.T) {
	expected := &Contract{ContractName: "Token", CompilerVersion: "v0.8.0", ABI: "[]"}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "getsourcecode", r.URL.Query().Get("action"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(apiResponse(t, []*Contract{expected}))
	})

	actual, err := c.GetContractSourceCode(context.Background(), "0xABC")
	require.NoError(t, err)
	assert.Equal(t, expected.ContractName, actual.ContractName)
	assert.Equal(t, expected.CompilerVersion, actual.CompilerVersion)
}

func TestGetContractSourceCode_EmptyResult(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(apiResponse(t, []*Contract{}))
	})

	_, err := c.GetContractSourceCode(context.Background(), "0xABC")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrContractSourceNotFound)
}

func TestGetContractSourceCode_APIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(apiErrorResponse("Contract not found"))
	})

	_, err := c.GetContractSourceCode(context.Background(), "0xBAD")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Contract not found")
}

// --- GetTransactionByHash ---

func TestGetTransactionByHash_Success(t *testing.T) {
	expected := &Tx{Hash: "0xdeadbeef", From: "0xAAA", To: "0xBBB"}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "gettxinfo", r.URL.Query().Get("action"))
		assert.Equal(t, "0xdeadbeef", r.URL.Query().Get("txhash"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(apiResponse(t, expected))
	})

	actual, err := c.GetTransactionByHash(context.Background(), "0xdeadbeef")
	require.NoError(t, err)
	assert.Equal(t, expected.Hash, actual.Hash)
	assert.Equal(t, expected.From, actual.From)
	assert.Equal(t, expected.To, actual.To)
}

func TestGetTransactionByHash_APIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(apiErrorResponse("Transaction not found"))
	})

	_, err := c.GetTransactionByHash(context.Background(), "0xBAD")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Transaction not found")
}

// --- Rate limiter middleware ---

type stubRateLimiter struct {
	limited bool
	err     error
	calls   int
}

func (s *stubRateLimiter) IsRateLimited(_ context.Context, _ string) (bool, error) {
	s.calls++
	return s.limited, s.err
}

func TestRateLimiterMiddleware_Passthrough(t *testing.T) {
	stub := &stubRateLimiter{limited: false}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(apiResponse(t, &Tx{Hash: "0x1"}))
	}, WithRateLimiter(stub))

	_, err := c.GetTransactionByHash(context.Background(), "0x1")
	require.NoError(t, err)
	assert.Equal(t, 1, stub.calls)
}

func TestRateLimiterMiddleware_Blocked(t *testing.T) {
	stub := &stubRateLimiter{limited: true}
	serverCalled := false
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		w.WriteHeader(http.StatusOK)
	}, WithRateLimiter(stub))

	_, err := c.GetTransactionByHash(context.Background(), "0x1")
	require.ErrorIs(t, err, ErrRateLimitAchieved)
	assert.False(t, serverCalled, "server must not be called when rate limited")
}

func TestRateLimiterMiddleware_Error(t *testing.T) {
	stub := &stubRateLimiter{err: assert.AnError}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, WithRateLimiter(stub))

	_, err := c.GetTransactionByHash(context.Background(), "0x1")
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestRateLimiterMiddleware_NilLimiterSkipped(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(apiResponse(t, &Tx{Hash: "0x1"}))
	})

	_, err := c.GetTransactionByHash(context.Background(), "0x1")
	require.NoError(t, err)
}

// --- isRetryable ---

func TestIsRetryable(t *testing.T) {
	assert.False(t, isRetryable(context.Canceled))
	assert.False(t, isRetryable(context.DeadlineExceeded))
	assert.True(t, isRetryable(&statusError{code: http.StatusTooManyRequests}))
	assert.False(t, isRetryable(&statusError{code: http.StatusNotFound}))
	assert.False(t, isRetryable(&statusError{code: http.StatusBadRequest}))
	assert.False(t, isRetryable(&statusError{code: http.StatusForbidden}))
}

// --- get: envelope parsing ---

func TestGet_InvalidJSON(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not-json`))
	})

	_, err := c.GetTransactionByHash(context.Background(), "0x1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid response")
}

// --- NewBlockscoutClient ---

func TestNewBlockscoutClient_DefaultTimeout(t *testing.T) {
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until the client times out.
		select {
		case <-r.Context().Done():
		case <-done:
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(func() { close(done); srv.Close() })

	c := NewBlockscoutClient(srv.URL, "key")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := c.GetTransactionByHash(ctx, "0x1")
	require.Error(t, err)
}
