package captcha

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// CapService verifies tokens with a self-hosted Cap Standalone instance.
type CapService struct {
	httpClient *http.Client
	verifyURL  string
	secretKey  string
}

// NewCapService creates a Cap verifier using the full site-key-specific siteverify URL.
func NewCapService(httpClient *http.Client, verifyURL, captchaSecretKey string) *CapService {
	client := *httpClient
	// Redirects must not forward the secret or token to a different endpoint.
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &CapService{
		httpClient: &client,
		verifyURL:  verifyURL,
		secretKey:  captchaSecretKey,
	}
}

// IsTokenValid posts a token to Cap and requires HTTP 200 with success: true.
func (c *CapService) IsTokenValid(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return ErrTokenIsNotProvided
	}

	//nolint:gosec // Cap's verification API requires the server secret in this request body.
	payload, err := json.Marshal(struct {
		Secret   string `json:"secret"`
		Response string `json:"response"`
	}{Secret: c.secretKey, Response: token})
	if err != nil {
		return fmt.Errorf("encode cap verification request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.verifyURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create cap verification request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request cap verification: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return ErrVerificationFailed
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read cap verification response: %w", err)
	}

	var result struct {
		Success bool `json:"success"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode cap verification response: %w", err)
	}

	if !result.Success {
		return ErrVerificationFailed
	}

	return nil
}
