package auth

import (
	"context"
	"fmt"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

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

	yandexID, err := uc.tokenService.ExchangeYandexCodeForID(ctx, uc.conf.YandexNew, req.Code)
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
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

	yandexID, err := uc.tokenService.ExchangeYandexCodeForID(ctx, uc.conf.YandexLogin, req.Code)
	if err != nil {
		return "", "", fmt.Errorf("failed to exchange code for id: %w", err)
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
