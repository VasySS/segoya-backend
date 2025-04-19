package token

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/lestrrat-go/jwx/v3/jwt"
	"golang.org/x/oauth2"
)

// ErrDiscordNoSubject is returned when there is no subject in the Discord token.
var ErrDiscordNoSubject = errors.New("failed to get subject from token")

// ExchangeDiscordCodeForID exchanges code from OAuth callback and gets a user's id in Discord.
func (s *Service) ExchangeDiscordCodeForID(ctx context.Context, config oauth2.Config, code string) (string, error) {
	token, err := config.Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("failed to exchange code: %w", err)
	}

	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		return "", fmt.Errorf("failed to get id token: %w", err)
	}

	jwtToken, err := jwt.ParseString(idToken, jwt.WithKeySet(s.jwkSet))
	if err != nil {
		slog.Debug("failed to parse token", slog.Any("error", err))
		return "", fmt.Errorf("failed to validate token: %w", err)
	}

	sub, ok := jwtToken.Subject()
	if !ok {
		return "", ErrDiscordNoSubject
	}

	return sub, nil
}
