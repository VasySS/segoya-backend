// Package captcha contains methods for working with captchas.
package captcha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const cloudflareCaptchaURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// ErrTokenIsNotProvided is returned when the token is not provided.
var ErrTokenIsNotProvided = errors.New("token is not provided")

// CloudflareService is a captcha checker service.
type CloudflareService struct {
	httpClient  *http.Client
	frontendURL string
	secretKey   string
}

// NewCloudflareService creates a new captcha checker service.
func NewCloudflareService(
	httpClient *http.Client,
	frontendURL, captchaSecretKey string,
) *CloudflareService {
	return &CloudflareService{
		httpClient:  httpClient,
		frontendURL: frontendURL,
		secretKey:   captchaSecretKey,
	}
}

// IsTokenValid checks if the token is valid by sending it to Cloudflare.
func (c *CloudflareService) IsTokenValid(ctx context.Context, token string) error {
	if strings.Contains(c.frontendURL, "localhost") || c.secretKey == "" {
		return nil
	}

	if token == "" {
		return ErrTokenIsNotProvided
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		cloudflareCaptchaURL,
		strings.NewReader(url.Values{
			"secret":   {c.secretKey},
			"response": {token},
		}.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making request to cloudflare: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body from cloudflare: %w", err)
	}

	type captchaResponse struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"` //nolint:tagliatelle
	}

	var respBody captchaResponse
	if err := json.Unmarshal(body, &respBody); err != nil {
		return fmt.Errorf("error unmarshalling captcha response from cloudflare: %w", err)
	}

	return nil
}
