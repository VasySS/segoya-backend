package captcha_test

import (
	"net/http"
	"testing"

	"github.com/VasySS/segoya-backend/pkg/captcha"
)

func TestYandexService_IsTokenValid(t *testing.T) {
	t.Parallel()
	testVerifier(t, verifierProvider{
		endpoint:  "https://smartcaptcha.cloud.yandex.ru/validate",
		field:     "token",
		success:   `{"status":"ok"}`,
		rejected:  `{"status":"failed"}`,
		expired:   `{"status":"failed","message":"Invalid or expired Token."}`,
		wrongType: `{"status":true}`,
		newService: func(client *http.Client, secret string) tokenVerifier {
			return captcha.NewYandexService(client, secret)
		},
	})
}
