package captcha

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const yandexCaptchaURL = "https://smartcaptcha.cloud.yandex.ru/validate"

// YandexService verifies Yandex SmartCaptcha tokens.
type YandexService struct {
	httpClient *http.Client
	secretKey  string
}

// NewYandexService creates a SmartCaptcha verification service.
func NewYandexService(httpClient *http.Client, captchaSecretKey string) *YandexService {
	return &YandexService{
		httpClient: httpClient,
		secretKey:  captchaSecretKey,
	}
}

// IsTokenValid checks a token with Yandex, without passing the optional client IP.
func (y *YandexService) IsTokenValid(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return ErrTokenIsNotProvided
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		yandexCaptchaURL,
		strings.NewReader(url.Values{
			"secret": {y.secretKey},
			"token":  {token},
		}.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := y.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making request to yandex: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return ErrVerificationFailed
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body from yandex: %w", err)
	}

	var result struct {
		Status string `json:"status"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("error unmarshalling captcha response from yandex: %w", err)
	}

	if result.Status != "ok" {
		return ErrVerificationFailed
	}

	return nil
}
