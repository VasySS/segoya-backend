package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"golang.org/x/oauth2"
)

// https://stackoverflow.com/questions/61850992/jwt-validation-with-jwks-golang
//
//nolint:ireturn
func (uc Usecase) newJWKSet(ctx context.Context, jwkURL string) jwk.Set {
	jwkCache := jwk.NewCache(ctx)

	// register a minimum refresh interval for this URL.
	// when not specified, defaults to Cache-Control and similar resp headers
	err := jwkCache.Register(jwkURL,
		jwk.WithMinRefreshInterval(10*time.Minute),
		jwk.WithHTTPClient(uc.conf.HTTPClientProxy),
	)
	if err != nil {
		panic("failed to register jwk location: " + err.Error())
	}

	// fetch once on application startup to check that url is valid
	_, err = jwkCache.Refresh(ctx, jwkURL)
	if err != nil {
		panic("error refreshing jwk: " + err.Error())
	}

	return jwk.NewCachedSet(jwkCache, jwkURL)
}

func (uc Usecase) discordExchangeCodeForID(ctx context.Context, config oauth2.Config, code string) (string, error) {
	token, err := config.Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("failed to exchange code: %w", err)
	}

	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		return "", fmt.Errorf("failed to get id token: %w", err)
	}

	jwt, err := jwt.ParseString(idToken, jwt.WithKeySet(
		uc.newJWKSet(ctx, "https://discord.com/api/oauth2/keys"),
	))
	if err != nil {
		return "", fmt.Errorf("failed to validate token: %w", err)
	}

	return jwt.Subject(), nil
}

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

	discordID, err := uc.discordExchangeCodeForID(ctx, uc.conf.DiscordNew, req.Code)
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

	discordID, err := uc.discordExchangeCodeForID(ctx, uc.conf.DiscordLogin, req.Code)
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
