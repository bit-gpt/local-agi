package serverwallet

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	coretypes "github.com/mudler/LocalAGI/core/types"
)

// HTTPClientWrapper provides a unified interface for HTTP requests
// that can use either H402 payment protocol or regular HTTP client
type HTTPClientWrapper struct {
	regularClient *http.Client
	h402Client    *H402Client
	useH402       bool
}

// HTTPResponseWithPaymentInfo wraps HTTP response with payment information
type HTTPResponseWithPaymentInfo struct {
	Response         *http.Response
	PaymentInfo      *PaymentInfo
	PayLimitExceeded *PayLimitError
	InfoError        error
}

// HTTPClientOptions configures the HTTP client wrapper
type HTTPClientOptions struct {
	Timeout            time.Duration
	InsecureSkipVerify bool
	DisableKeepAlives  bool
	ForceHTTP1         bool
	ServerWallets      map[coretypes.ServerWalletType]coretypes.ServerWallet
	PayLimits          map[string]float64
	AgentID            uuid.UUID
	Pool               interface{} // Using interface{} to avoid circular import
}

// NewHTTPClientWrapper creates a new HTTP client wrapper
// It automatically detects whether to use H402 based on LOCALAGI_ENABLE_SERVER_WALLETS environment variable
func NewHTTPClientWrapper(opts HTTPClientOptions) *HTTPClientWrapper {

	// Set default timeout if not provided
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}

	// Create regular HTTP client
	transport := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: opts.InsecureSkipVerify},
		DisableKeepAlives: opts.DisableKeepAlives,
		ForceAttemptHTTP2: !opts.ForceHTTP1,
	}

	regularClient := &http.Client{
		Timeout:   opts.Timeout,
		Transport: transport,
	}

	wrapper := &HTTPClientWrapper{
		regularClient: regularClient,
		useH402:       os.Getenv("LOCALAGI_ENABLE_SERVER_WALLETS") == "true",
	}

	// Initialize H402 client if server wallets are enabled and wallets are provided
	if wrapper.useH402 && len(opts.ServerWallets) > 0 {
		wrapper.h402Client = NewH402ClientWithWallets(opts.ServerWallets, opts.PayLimits, opts.AgentID, opts.Pool)
	} else {
		// Fall back to regular client if H402 is not enabled or no wallets provided
		wrapper.useH402 = false
	}

	return wrapper
}

// NewDefaultHTTPClientWrapper creates a wrapper with default settings
func NewDefaultHTTPClientWrapper() *HTTPClientWrapper {
	return NewHTTPClientWrapper(HTTPClientOptions{})
}

// DoWithPaymentInfo performs an HTTP request and returns payment information if available
func (w *HTTPClientWrapper) DoWithPaymentInfo(req *http.Request) (*HTTPResponseWithPaymentInfo, error) {
	if w.useH402 && w.h402Client != nil {
		// For H402, we need the request body to send the payment request
		var requestBody []byte
		if req.Body != nil {
			var err error
			requestBody, err = io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			req.Body.Close()

			// Restore the body for the request
			req.Body = io.NopCloser(bytes.NewReader(requestBody))
		}

		h402Result, err := w.h402Client.SendH402RequestWithPaymentInfo(req.Context(), req.Method, req.URL.String(), requestBody)
		if err != nil {
			return nil, err
		}

		return &HTTPResponseWithPaymentInfo{
			Response:         h402Result.Response,
			PaymentInfo:      h402Result.PaymentInfo,
			PayLimitExceeded: h402Result.PayLimitExceeded,
			InfoError:        h402Result.InfoError,
		}, nil
	}

	resp, err := w.regularClient.Do(req)
	if err != nil {
		return nil, err
	}

	return &HTTPResponseWithPaymentInfo{
		Response:         resp,
		PaymentInfo:      nil,
		PayLimitExceeded: nil,
		InfoError:        nil,
	}, nil
}

// Do performs an HTTP request using the appropriate client (H402 or regular)
func (w *HTTPClientWrapper) Do(req *http.Request) (*http.Response, error) {
	result, err := w.DoWithPaymentInfo(req)
	if err != nil {
		return nil, err
	}
	return result.Response, nil
}

// Post performs a POST request
func (w *HTTPClientWrapper) Post(ctx context.Context, url string, contentType string, body []byte) (*http.Response, error) {
	if w.useH402 && w.h402Client != nil {
		return w.h402Client.SendH402Request(ctx, "POST", url, body)
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bodyReader)
	if err != nil {
		return nil, err
	}

	if body != nil && contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	return w.regularClient.Do(req)
}

// Get performs a GET request
func (w *HTTPClientWrapper) Get(ctx context.Context, url string) (*http.Response, error) {
	if w.useH402 && w.h402Client != nil {
		return w.h402Client.SendH402Request(ctx, "GET", url, nil)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	return w.regularClient.Do(req)
}

// GetRegularClient returns the underlying regular HTTP client
// This can be useful for cases where direct access is needed
func (w *HTTPClientWrapper) GetRegularClient() *http.Client {
	return w.regularClient
}

// GetH402Client returns the H402 client if available
func (w *HTTPClientWrapper) GetH402Client() *H402Client {
	return w.h402Client
}

// IsH402Enabled returns whether H402 is currently enabled
func (w *HTTPClientWrapper) IsH402Enabled() bool {
	return w.useH402 && w.h402Client != nil
}
