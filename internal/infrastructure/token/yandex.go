package token

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"golang.org/x/oauth2"
)

// ErrYandexNotAvailable is returned when Yandex OAuth provider is not available.
var ErrYandexNotAvailable = errors.New("yandex is not available")

// ExchangeYandexCodeForID exchanges code from OAuth callback and gets a user's id in Yandex.
func (s *Service) ExchangeYandexCodeForID(ctx context.Context, config oauth2.Config, code string) (string, error) {
	oauthToken, err := config.Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("failed to exchange code for token: %w", err)
	}

	accessToken := oauthToken.AccessToken

	return s.yandexExchangeTokenForID(accessToken)
}

func (s *Service) yandexExchangeTokenForID(tokenStr string) (string, error) {
	resp, err := s.httpClient.Do(&http.Request{
		Method: http.MethodGet,
		URL: &url.URL{
			Scheme:   "https",
			Host:     "login.yandex.ru",
			Path:     "/info",
			RawQuery: "format=jwt",
		},
		Header: map[string][]string{
			"Authorization": {"OAuth " + tokenStr},
		},
	})
	if err != nil {
		return "", fmt.Errorf("error getting id from yandex: %w", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", ErrYandexNotAvailable
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading body from yandex: %w", err)
	}

	token, err := jwt.ParseString(string(body), jwt.WithKey(jwa.HS256(), []byte(s.yandexSecretKey)))
	if err != nil {
		return "", fmt.Errorf("failed to parse token: %w", err)
	}

	var uid float64

	if err := token.Get("uid", &uid); err != nil {
		return "", fmt.Errorf("failed to get uid from token: %w", err)
	}

	return fmt.Sprintf("%.0f", uid), nil
}
