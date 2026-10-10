package captcha_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/pkg/captcha"
	httpPkg "github.com/VasySS/segoya-backend/pkg/http"
)

func TestCapService_IsTokenValid(t *testing.T) {
	t.Parallel()
	testVerifier(t, verifierProvider{
		endpoint:    "http://cap:3000/site-key/siteverify",
		field:       "response",
		jsonRequest: true,
		success:     `{"success":true}`,
		rejected:    `{"success":false}`,
		expired:     `{"success":false,"error":"Invalid token"}`,
		wrongType:   `{"success":"true"}`,
		newService: func(client *http.Client, secret string) tokenVerifier {
			return captcha.NewCapService(client, "http://cap:3000/site-key/siteverify", secret)
		},
	})
}

func TestCapService_RejectsRedirects(t *testing.T) {
	t.Parallel()

	var redirectedCalls atomic.Int32

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirectedCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()

	client := httpPkg.NewClient()
	defer client.CloseIdleConnections()

	service := captcha.NewCapService(client, source.URL, "secret")
	require.ErrorIs(t, service.IsTokenValid(t.Context(), "token"), captcha.ErrVerificationFailed)
	assert.Zero(t, redirectedCalls.Load(), "redirects must not forward credentials or tokens")

	// The shared client retains its existing redirect behavior for other integrations.
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, source.URL, nil)
	require.NoError(t, err)
	response, err := client.Do(req)
	require.NoError(t, err)

	defer response.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.EqualValues(t, 1, redirectedCalls.Load())
}

func TestCapService_InvalidEndpoint(t *testing.T) {
	t.Parallel()

	service := captcha.NewCapService(httpPkg.NewClient(), "://invalid", "secret")
	require.Error(t, service.IsTokenValid(t.Context(), "token"))
}
