package captcha_test

import (
	"net/http"
	"testing"

	"github.com/VasySS/segoya-backend/pkg/captcha"
)

func TestCloudflareService_IsTokenValid(t *testing.T) {
	t.Parallel()
	testVerifier(t, verifierProvider{
		endpoint:  "https://challenges.cloudflare.com/turnstile/v0/siteverify",
		field:     "response",
		success:   `{"success":true}`,
		rejected:  `{"success":false}`,
		expired:   `{"success":false,"error-codes":["timeout-or-duplicate"]}`,
		wrongType: `{"success":"true"}`,
		newService: func(client *http.Client, secret string) tokenVerifier {
			return captcha.NewCloudflareService(client, secret)
		},
	})
}
