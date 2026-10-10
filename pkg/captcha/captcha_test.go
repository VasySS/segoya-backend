package captcha_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/pkg/captcha"
	httpPkg "github.com/VasySS/segoya-backend/pkg/http"
)

type tokenVerifier interface {
	IsTokenValid(ctx context.Context, token string) error
}

type verifierProvider struct {
	endpoint   string
	field      string
	success    string
	rejected   string
	expired    string
	wrongType  string
	newService func(*http.Client, string) tokenVerifier
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type trackedBody struct {
	io.Reader

	closed bool
}

func (b *trackedBody) Close() error {
	b.closed = true
	return nil
}

type verifierCase struct {
	name         string
	body         string
	status       int
	emptyToken   bool
	blankToken   bool
	emptySecret  bool
	transportErr error
	readErr      error
	cancel       bool
	timeout      bool
	wantErr      bool
	wantCause    error
}

func testVerifier(t *testing.T, provider verifierProvider) {
	t.Helper()

	transportErr := errors.New("transport failure")
	readErr := errors.New("body read failure")
	tests := []verifierCase{
		{name: "success", body: provider.success},
		{name: "empty secret still verifies", body: provider.success, emptySecret: true},
		{name: "rejected", body: provider.rejected, wantErr: true, wantCause: captcha.ErrVerificationFailed},
		{name: "expired token", body: provider.expired, wantErr: true, wantCause: captcha.ErrVerificationFailed},
		{name: "reused token", body: provider.expired, wantErr: true, wantCause: captcha.ErrVerificationFailed},
		{name: "empty token", emptyToken: true, wantErr: true, wantCause: captcha.ErrTokenIsNotProvided},
		{name: "whitespace token", blankToken: true, wantErr: true, wantCause: captcha.ErrTokenIsNotProvided},
		{name: "missing success field", body: `{}`, wantErr: true, wantCause: captcha.ErrVerificationFailed},
		{name: "null response", body: `null`, wantErr: true, wantCause: captcha.ErrVerificationFailed},
		{name: "wrong success type", body: provider.wrongType, wantErr: true},
		{name: "empty body", wantErr: true},
		{name: "malformed json", body: `{`, wantErr: true},
		{name: "trailing garbage", body: provider.success + "invalid", wantErr: true},
		{name: "multiple json values", body: provider.success + provider.success, wantErr: true},
		{
			name: "non-200 success body", status: http.StatusServiceUnavailable, body: provider.success,
			wantErr: true, wantCause: captcha.ErrVerificationFailed,
		},
		{
			name: "non-200 rejection", status: http.StatusBadRequest, body: provider.rejected,
			wantErr: true, wantCause: captcha.ErrVerificationFailed,
		},
		{name: "transport error", transportErr: transportErr, wantErr: true, wantCause: transportErr},
		{name: "body read error", readErr: readErr, wantErr: true, wantCause: readErr},
		{name: "cancellation", cancel: true, wantErr: true, wantCause: context.Canceled},
		{name: "client timeout", timeout: true, wantErr: true, wantCause: context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			testVerifierCase(t, provider, tt)
		})
	}

	t.Run("success is never cached", func(t *testing.T) {
		t.Parallel()

		var calls int

		client := httpPkg.NewClient()
		client.Transport = roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			calls++

			body := provider.success
			if calls > 1 {
				body = provider.expired
			}

			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		service := provider.newService(client, "secret")
		require.NoError(t, service.IsTokenValid(t.Context(), "same-token"))
		require.ErrorIs(t, service.IsTokenValid(t.Context(), "same-token"), captcha.ErrVerificationFailed)
		assert.Equal(t, 2, calls)
	})
}

func testVerifierCase(t *testing.T, provider verifierProvider, tt verifierCase) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	secret, token := tt.input()

	body := &trackedBody{Reader: strings.NewReader(tt.body)}
	if tt.readErr != nil {
		body.Reader = iotest.ErrReader(tt.readErr)
	}

	status := tt.status
	if status == 0 {
		status = http.StatusOK
	}

	var calls int

	client := httpPkg.NewClient()
	assert.Equal(t, 5*time.Second, client.Timeout)

	if tt.timeout {
		client.Timeout = time.Millisecond
	}

	client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++

		assert.Equal(t, provider.endpoint, req.URL.String())
		assert.Empty(t, req.URL.RawQuery)
		assert.Equal(t, http.MethodPost, req.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
		require.NoError(t, req.ParseForm())
		assert.Equal(t, url.Values{"secret": {secret}, provider.field: {token}}, req.PostForm)
		assert.Equal(t, ctx.Value(contextKey{}), req.Context().Value(contextKey{}))

		return verifierResponse(req.Context(), cancel, tt, &http.Response{ //nolint:contextcheck // Use the client deadline.
			StatusCode: status, Body: body, Header: http.Header{},
		})
	})
	ctx = context.WithValue(ctx, contextKey{}, "request metadata")

	err := provider.newService(client, secret).IsTokenValid(ctx, token)
	assertVerifierResult(t, tt, err)

	if tt.emptyToken || tt.blankToken {
		assert.Zero(t, calls)
		return
	}

	assert.Equal(t, 1, calls, "verification must never retry")

	if !tt.cancel && !tt.timeout && tt.transportErr == nil {
		assert.True(t, body.closed, "response body must be closed on every response path")
	}
}

func (tt verifierCase) input() (string, string) {
	secret, token := "secret+&= /", "token+&= /"
	if tt.emptySecret {
		secret = ""
	}

	if tt.emptyToken {
		token = ""
	}

	if tt.blankToken {
		token = " \t\n"
	}

	return secret, token
}

func verifierResponse(
	ctx context.Context,
	cancel context.CancelFunc,
	tt verifierCase,
	response *http.Response,
) (*http.Response, error) {
	if tt.cancel {
		cancel()
	}

	if tt.cancel || tt.timeout {
		<-ctx.Done()
		return nil, fmt.Errorf("mock request canceled: %w", ctx.Err())
	}

	if tt.transportErr != nil {
		return nil, tt.transportErr
	}

	return response, nil
}

func assertVerifierResult(t *testing.T, tt verifierCase, err error) {
	t.Helper()

	if tt.wantErr {
		require.Error(t, err)
	} else {
		require.NoError(t, err)
	}

	if tt.wantCause != nil {
		require.ErrorIs(t, err, tt.wantCause)
	}
}

type contextKey struct{}
