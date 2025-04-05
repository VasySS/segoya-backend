package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"golang.org/x/oauth2"
)

var (
	// ErrYandexNotAvailable is returned when Yandex oauth provider is not available.
	ErrYandexNotAvailable = errors.New("yandex is not available")
)

func yandexExchangeCodeForToken(ctx context.Context, config oauth2.Config, code string) (string, error) {
	token, err := config.Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("failed to exchange code for token: %w", err)
	}

	if !token.Valid() {
		return "", fmt.Errorf("failed to validate token: %w", err)
	}

	return token.AccessToken, nil
}

func (uc Usecase) yandexExchangeTokenForID(tokenStr, yandexSecretKey string) (string, error) {
	resp, err := uc.conf.HTTPClientProxy.Do(&http.Request{
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

	token, err := jwt.ParseString(string(body), jwt.WithKey(jwa.HS256, []byte(yandexSecretKey)))
	if err != nil {
		return "", fmt.Errorf("failed to parse token: %w", err)
	}

	uid, _ := token.Get("uid")
	uidFloat, _ := uid.(float64)

	return fmt.Sprintf("%.0f", uidFloat), nil
}

// NewYandex saves state and userID associated with it in db.
func (uc Usecase) NewYandex(ctx context.Context, req dto.NewOAuthRequest) error {
	ctx, span := uc.tracer.Start(ctx, "NewYandex")
	defer span.End()

	if err := uc.sessionRepo.NewOAuthState(ctx, req); err != nil {
		return fmt.Errorf("failed to add oauth info in db: %w", err)
	}

	return nil
}

// NewYandexCallback exchanges code from oauth callback to get oauth id and creates new link in db.
func (uc Usecase) NewYandexCallback(ctx context.Context, req dto.NewOAuthCallbackRequest) error {
	ctx, span := uc.tracer.Start(ctx, "NewYandexCallback")
	defer span.End()

	userID, err := uc.sessionRepo.GetOAuthUserID(ctx, req.State)
	if err != nil {
		return fmt.Errorf("failed to get user id: %w", err)
	}

	yandexToken, err := yandexExchangeCodeForToken(ctx, uc.conf.YandexNew, req.Code)
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}

	yandexID, err := uc.yandexExchangeTokenForID(yandexToken, uc.conf.YandexSecretKey)
	if err != nil {
		return fmt.Errorf("failed to exchange token: %w", err)
	}

	dbReq := dto.NewOAuthRequestDB{
		RequestTime: req.RequestTime,
		OAuthID:     yandexID,
		UserID:      userID,
		Issuer:      user.YandexOAuth,
	}

	if err := uc.userRepo.NewOAuth(ctx, dbReq); err != nil {
		return fmt.Errorf("failed to add oauth info in db: %w", err)
	}

	return nil
}

// LoginYandexCallback exchanges code from oauth callback for access token and refresh token.
func (uc Usecase) LoginYandexCallback(ctx context.Context, req dto.OAuthLoginCallbackRequest) (string, string, error) {
	ctx, span := uc.tracer.Start(ctx, "LoginYandexCallback")
	defer span.End()

	yandexToken, err := yandexExchangeCodeForToken(ctx, uc.conf.YandexLogin, req.Code)
	if err != nil {
		return "", "", err
	}

	yandexID, err := uc.yandexExchangeTokenForID(yandexToken, uc.conf.YandexSecretKey)
	if err != nil {
		return "", "", err
	}

	userDB, err := uc.userRepo.GetUserByOAuth(ctx, dto.GetUserByOAuthRequest{
		OAuthID: yandexID,
		Issuer:  user.YandexOAuth,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to get user: %w", err)
	}

	sessionID := uc.cryptoService.NewUUID4()

	accessToken, err := uc.tokenService.NewAccessToken(req.RequestTime, user.AccessTokenClaims{
		SessionID: sessionID,
		UserID:    userDB.ID,
		Username:  userDB.Username,
		Name:      userDB.Name,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to create access token: %w", err)
	}

	refreshToken, err := uc.tokenService.NewRefreshToken(req.RequestTime, user.RefreshTokenClaims{
		SessionID: sessionID,
		UserID:    userDB.ID,
		Username:  userDB.Username,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to create refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

// DeleteYandex deletes oauth connection for user.
func (uc Usecase) DeleteYandex(ctx context.Context, req dto.DeleteOAuthRequest) error {
	ctx, span := uc.tracer.Start(ctx, "DeleteYandex")
	defer span.End()

	if err := uc.userRepo.DeleteOAuth(ctx, req); err != nil {
		return fmt.Errorf("failed to delete oauth info in db: %w", err)
	}

	return nil
}
