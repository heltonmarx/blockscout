package blockscout

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/avast/retry-go/v5"
)

type HTTPClient struct {
	rawurl string
	client *http.Client
}

func NewHTTPClient(rawurl string, timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		rawurl: rawurl,
		client: &http.Client{
			Transport: defaultTransport(timeout),
			Timeout:   timeout,
		},
	}
}

// Send sends an HTTP request with the specified method and body. It supports optional retry options.
// If no retry options are provided, it sends the request once.
// If retry options are provided, it uses them to retry the request upon failure.
func (c *HTTPClient) Send(ctx context.Context, method, path string, body []byte, opts ...retry.Option) (buf []byte, err error) {
	if len(opts) == 0 {
		return c.send(ctx, method, path, body)
	}
	return retry.NewWithData[[]byte](opts...).Do(func() ([]byte, error) {
		return c.send(ctx, method, path, body)
	})
}

func (c *HTTPClient) Get(ctx context.Context, path string, opts ...retry.Option) ([]byte, error) {
	return c.Send(ctx, http.MethodGet, path, nil, opts...)
}

func (c *HTTPClient) Post(ctx context.Context, path string, body []byte, opts ...retry.Option) ([]byte, error) {
	return c.Send(ctx, http.MethodPost, path, body, opts...)
}

func (c *HTTPClient) send(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	if c.rawurl == "" {
		return nil, errors.New("HTTPClient: rawurl is empty")
	}

	host, err := url.Parse(c.rawurl)
	if err != nil {
		return nil, err
	}
	host.RawQuery = path

	req, err := http.NewRequestWithContext(ctx, method, host.String(), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if isUnrecoverable(resp.StatusCode) {
		return nil, retry.Unrecoverable(fmt.Errorf("unrecoverable error: %s(%d)", http.StatusText(resp.StatusCode), resp.StatusCode))
	}
	if !is200Range(resp.StatusCode) {
		return nil, &statusError{code: resp.StatusCode}
	}
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(buf) == 0 {
		return nil, errors.New("empty response body")
	}
	return buf, nil
}

type statusError struct {
	code int
}

func (e *statusError) Error() string {
	return fmt.Sprintf("unexpected status code: %s(%d)", http.StatusText(e.code), e.code)
}

func isUnrecoverable(code int) bool {
	if code == http.StatusTooManyRequests {
		return false // 429 — serverr rate limit, back off and retry
	}
	if code == http.StatusNotImplemented {
		return true
	}
	if code >= http.StatusInternalServerError {
		return false
	}
	return code >= http.StatusBadRequest
}

func is200Range(code int) bool {
	return code >= http.StatusOK && code <= http.StatusIMUsed
}

func defaultTransport(timeout time.Duration) *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		Proxy:                 http.ProxyFromEnvironment,
		ResponseHeaderTimeout: 5 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}
}
