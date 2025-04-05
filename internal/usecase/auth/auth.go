package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// Login authenticates a user and generates new access and refresh tokens.
func (uc Usecase) Login(ctx context.Context, req dto.LoginRequest) (string, string, error) {
	ctx, span := uc.tracer.Start(ctx, "Login")
	defer span.End()

	userDB, err := uc.userRepo.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return "", "", fmt.Errorf("failed to get user from db: %w", err)
	}

	if err := uc.cryptoService.CompareHashAndPassword(userDB.Password, req.Password); err != nil {
		return "", "", user.ErrWrongPassword
	}

	sessionID := uc.cryptoService.NewUUID4()

	accessToken, err := uc.tokenService.NewAccessToken(req.RequestTime, user.AccessTokenClaims{
		SessionID: sessionID,
		UserID:    userDB.ID,
		Username:  userDB.Username,
		Name:      userDB.Name,
	})
	if err != nil {
		return "", "", fmt.Errorf("error creating access token: %w", err)
	}

	refreshToken, err := uc.tokenService.NewRefreshToken(req.RequestTime, user.RefreshTokenClaims{
		SessionID: sessionID,
		UserID:    userDB.ID,
		Username:  userDB.Username,
	})
	if err != nil {
		return "", "", fmt.Errorf("error creating refresh token: %w", err)
	}

	if err := uc.sessionRepo.NewSession(ctx, dto.NewSessionRequest{
		RequestTime:  req.RequestTime,
		UserID:       userDB.ID,
		SessionID:    sessionID,
		RefreshToken: refreshToken,
		UA:           req.UserAgent,
		Expiration:   uc.conf.RefreshTokenTTL,
	}); err != nil {
		return "", "", fmt.Errorf("error creating user session: %w", err)
	}

	return accessToken, refreshToken, nil
}

// Register creates a new user account.
func (uc Usecase) Register(ctx context.Context, req dto.RegisterRequest) error {
	ctx, span := uc.tracer.Start(ctx, "Register")
	defer span.End()

	_, err := uc.userRepo.GetUserByUsername(ctx, req.Username)
	if !errors.Is(err, user.ErrUserNotFound) {
		return fmt.Errorf("failed to get user from db: %w", err)
	}

	passwordHash, err := uc.cryptoService.GenerateHashFromPassword(req.Password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	if err := uc.userRepo.NewUser(ctx, dto.RegisterRequestDB{
		RequestTime: req.RequestTime,
		Username:    req.Username,
		Name:        req.Name,
		Password:    passwordHash,
	}); err != nil {
		return fmt.Errorf("failed to create user in db: %w", err)
	}

	return nil
}

// RefreshTokens generates new access and refresh tokens.
// It checks that a session exists for provided refresh token and updates session expiration.
func (uc Usecase) RefreshTokens(ctx context.Context, req dto.TokensRefreshRequest) (string, string, error) {
	ctx, span := uc.tracer.Start(ctx, "RefreshTokens")
	defer span.End()

	tokenClaims, err := uc.tokenService.ParseRefreshToken(req.RefreshToken)
	if err != nil {
		return "", "", fmt.Errorf("error parsing refresh token: %w", err)
	}

	// check that session still exists
	if _, err := uc.sessionRepo.GetSession(ctx, tokenClaims.UserID, tokenClaims.SessionID); err != nil {
		return "", "", fmt.Errorf("error getting user session from db: %w", err)
	}

	userDB, err := uc.userRepo.GetUserByUsername(ctx, tokenClaims.Username)
	if err != nil {
		return "", "", fmt.Errorf("error getting user from db: %w", err)
	}

	newAccessToken, err := uc.tokenService.NewAccessToken(req.RequestTime, user.AccessTokenClaims{
		SessionID: tokenClaims.SessionID,
		UserID:    userDB.ID,
		Username:  userDB.Username,
		Name:      userDB.Name,
	})
	if err != nil {
		return "", "", fmt.Errorf("error creating access token: %w", err)
	}

	newRefreshToken, err := uc.tokenService.NewRefreshToken(req.RequestTime, user.RefreshTokenClaims{
		SessionID: tokenClaims.SessionID,
		UserID:    userDB.ID,
		Username:  userDB.Username,
	})
	if err != nil {
		return "", "", fmt.Errorf("error creating refresh token: %w", err)
	}

	if err := uc.sessionRepo.UpdateSession(ctx, dto.UpdateSessionRequest{
		RequestTime:  req.RequestTime,
		UserID:       userDB.ID,
		SessionID:    tokenClaims.SessionID,
		RefreshToken: newRefreshToken,
		Expiration:   uc.conf.RefreshTokenTTL,
	}); err != nil {
		return "", "", fmt.Errorf("error refreshing user session: %w", err)
	}

	return newAccessToken, newRefreshToken, nil
}

// GetOAuth retrieves all linked OAuth providers for a user.
func (uc Usecase) GetOAuth(ctx context.Context, userID int) ([]user.OAuth, error) {
	ctx, span := uc.tracer.Start(ctx, "GetOAuth")
	defer span.End()

	providers, err := uc.userRepo.GetOAuth(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get oauth info: %w", err)
	}

	return providers, nil
}

// GetSessions retrieves all active sessions for a user.
func (uc Usecase) GetSessions(ctx context.Context, userID int) ([]user.Session, error) {
	ctx, span := uc.tracer.Start(ctx, "GetSessions")
	defer span.End()

	sessions, err := uc.sessionRepo.GetSessions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}

	return sessions, nil
}

// DeleteSession removes a specific authentication session.
func (uc Usecase) DeleteSession(ctx context.Context, userID int, sessionID string) error {
	ctx, span := uc.tracer.Start(ctx, "DeleteSession")
	defer span.End()

	session, err := uc.sessionRepo.GetSession(ctx, userID, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get user session: %w", err)
	}

	if session.UserID != userID {
		return user.ErrSessionWrongUser
	}

	if err := uc.sessionRepo.DeleteSession(ctx, userID, sessionID); err != nil {
		return fmt.Errorf("failed to delete user session: %w", err)
	}

	return nil
}
