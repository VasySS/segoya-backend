package auth

import (
	"context"
	"fmt"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// NewDiscord saves state and userID associated with it in db.
func (uc Usecase) NewDiscord(ctx context.Context, req dto.NewOAuthRequest) error {
	ctx, span := uc.tracer.Start(ctx, "NewDiscord")
	defer span.End()

	if err := uc.sessionRepo.NewOAuthState(ctx, req); err != nil {
		return fmt.Errorf("failed to add oauth info in db: %w", err)
	}

	return nil
}

// NewDiscordCallback exchanges code from oauth callback to get oauth id and creates new link in db.
func (uc Usecase) NewDiscordCallback(ctx context.Context, req dto.NewOAuthCallbackRequest) error {
	ctx, span := uc.tracer.Start(ctx, "NewDiscordCallback")
	defer span.End()

	userID, err := uc.sessionRepo.GetOAuthUserID(ctx, req.State)
	if err != nil {
		return fmt.Errorf("failed to get user id: %w", err)
	}

	discordID, err := uc.tokenService.ExchangeDiscordCodeForID(ctx, uc.conf.DiscordNew, req.Code)
	if err != nil {
		return fmt.Errorf("failed to exchange code for id: %w", err)
	}

	dbReq := dto.NewOAuthRequestDB{
		RequestTime: req.RequestTime,
		OAuthID:     discordID,
		UserID:      userID,
		Issuer:      user.DiscordOAuth,
	}

	if err := uc.userRepo.NewOAuth(ctx, dbReq); err != nil {
		return fmt.Errorf("failed to add oauth info in db: %w", err)
	}

	return nil
}

// LoginDiscordCallback exchanges code from oauth callback for access token and refresh token.
func (uc Usecase) LoginDiscordCallback(ctx context.Context, req dto.OAuthLoginCallbackRequest) (string, string, error) {
	ctx, span := uc.tracer.Start(ctx, "LoginDiscordCallback")
	defer span.End()

	discordID, err := uc.tokenService.ExchangeDiscordCodeForID(ctx, uc.conf.DiscordLogin, req.Code)
	if err != nil {
		return "", "", fmt.Errorf("failed to exchange code for id: %w", err)
	}

	userDB, err := uc.userRepo.GetUserByOAuth(ctx, dto.GetUserByOAuthRequest{
		OAuthID: discordID,
		Issuer:  user.DiscordOAuth,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to get user from db: %w", err)
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

// DeleteDiscord deletes oauth connection for user.
func (uc Usecase) DeleteDiscord(ctx context.Context, req dto.DeleteOAuthRequest) error {
	ctx, span := uc.tracer.Start(ctx, "DeleteDiscord")
	defer span.End()

	if err := uc.userRepo.DeleteOAuth(ctx, req); err != nil {
		return fmt.Errorf("failed to delete oauth info from db: %w", err)
	}

	return nil
}
