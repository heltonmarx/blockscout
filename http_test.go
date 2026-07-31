package blockscout

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHTTPClient(t *testing.T, handler http.HandlerFunc) (*HTTPClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewHTTPClient(srv.URL, 3*time.Second), srv
}

func TestHTTPClient_Get_Success(t *testing.T) {
	c, _ := newTestHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"1"}`))
	})

	buf, err := c.Get(context.Background(), "")
	require.NoError(t, err)
	assert.JSONEq(t, `{"status":"1"}`, string(buf))
}

func TestHTTPClient_Post_Success(t *testing.T) {
	c, _ := newTestHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	buf, err := c.Post(context.Background(), "", []byte(`{"x":1}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(buf))
}

func TestHTTPClient_QueryStringForwarded(t *testing.T) {
	c, _ := newTestHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "foo=bar", r.URL.RawQuery)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	_, err := c.Get(context.Background(), "foo=bar")
	require.NoError(t, err)
}

func TestHTTPClient_EmptyRawURL(t *testing.T) {
	c := NewHTTPClient("", 3*time.Second)
	_, err := c.Get(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rawurl is empty")
}

func TestHTTPClient_EmptyResponseBody(t *testing.T) {
	c, _ := newTestHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	_, err := c.Get(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty response body")
}

func TestHTTPClient_Non2xxIsError(t *testing.T) {
	c, _ := newTestHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := c.Get(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestHTTPClient_5xxIsRetried(t *testing.T) {
	calls := 0
	c, _ := newTestHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	buf, err := c.Get(context.Background(), "", retryTestOpts()...)
	require.NoError(t, err)
	assert.Equal(t, 3, calls, "should have retried on 500")
	assert.NotEmpty(t, buf)
}

func TestHTTPClient_501IsUnrecoverable(t *testing.T) {
	calls := 0
	c, _ := newTestHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotImplemented)
	})

	_, err := c.Get(context.Background(), "", retryTestOpts()...)
	require.Error(t, err)
	assert.Equal(t, 1, calls, "501 should stop retries immediately")
}

func TestHTTPClient_429IsRetried(t *testing.T) {
	calls := 0
	c, _ := newTestHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	buf, err := c.Get(context.Background(), "", retryTestOpts()...)
	require.NoError(t, err)
	assert.Equal(t, 3, calls, "429 should be retried")
	assert.NotEmpty(t, buf)
}

func TestHTTPClient_ContextCancellation(t *testing.T) {
	c, _ := newTestHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Get(ctx, "")
	require.Error(t, err)
}

func TestIsUnrecoverable(t *testing.T) {
	// 4xx (except 429 — same rule) are unrecoverable client errors
	assert.True(t, isUnrecoverable(http.StatusBadRequest))
	assert.True(t, isUnrecoverable(http.StatusUnauthorized))
	assert.True(t, isUnrecoverable(http.StatusForbidden))
	assert.True(t, isUnrecoverable(http.StatusNotFound))
	assert.False(t, isUnrecoverable(http.StatusTooManyRequests))
	// 501 is the only 5xx that is unrecoverable
	assert.True(t, isUnrecoverable(http.StatusNotImplemented))
	// 5xx (except 501) are transient and retryable
	assert.False(t, isUnrecoverable(http.StatusInternalServerError))
	assert.False(t, isUnrecoverable(http.StatusBadGateway))
	assert.False(t, isUnrecoverable(http.StatusServiceUnavailable))
	assert.False(t, isUnrecoverable(http.StatusGatewayTimeout))
	// 2xx are not errors at all
	assert.False(t, isUnrecoverable(http.StatusOK))
}

func TestIs200Range(t *testing.T) {
	assert.True(t, is200Range(http.StatusOK))
	assert.True(t, is200Range(http.StatusCreated))
	assert.True(t, is200Range(http.StatusNoContent))
	assert.False(t, is200Range(http.StatusMovedPermanently))
	assert.False(t, is200Range(http.StatusBadRequest))
	assert.False(t, is200Range(http.StatusInternalServerError))
}
