package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// Login validates the captcha token, authenticates the user with the provided credentials,
// and generates a new access and refresh token pair if successful.
func (h Handler) Login(
	ctx context.Context,
	req *api.LoginReq,
	params api.LoginParams,
) (api.LoginRes, error) {
	if err := h.cs.IsTokenValid(ctx, params.XCaptchaToken.Value); err != nil {
		return &api.LoginBadRequest{
			Title:  "Captcha validation failed",
			Status: http.StatusBadRequest,
			Detail: "Captcha validation failed, please try again",
		}, nil
	}

	accessToken, refreshToken, err := h.uc.Login(ctx, dto.LoginRequest{
		RequestTime: time.Now().UTC(),
		Username:    req.Username,
		Password:    req.Password,
		UserAgent:   params.UserAgent,
	})
	if errors.Is(err, user.ErrUserNotFound) || errors.Is(err, user.ErrWrongPassword) {
		return &api.LoginUnauthorized{
			Title:  "Wrong credentials",
			Status: http.StatusUnauthorized,
			Detail: "Wrong username or password",
		}, nil
	} else if err != nil {
		slog.Error("error user login", slog.Any("error", err))

		return &api.LoginInternalServerError{
			Title:  "Error authorizing user",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	return &api.LoginNoContent{
		SetCookie: h.newCookieStringFromTokens(accessToken, refreshToken),
	}, nil
}

// Register validates the captcha token, creates a new user account.
func (h Handler) Register(
	ctx context.Context,
	req *api.RegisterReq,
	params api.RegisterParams,
) (api.RegisterRes, error) {
	if err := h.cs.IsTokenValid(ctx, params.XCaptchaToken.Value); err != nil {
		return &api.RegisterBadRequest{
			Title:  "Captcha validation failed",
			Status: http.StatusBadRequest,
			Detail: "Captcha validation failed, please try again",
		}, nil
	}

	err := h.uc.Register(ctx, dto.RegisterRequest{
		RequestTime: time.Now().UTC(),
		Username:    req.Username,
		Password:    req.Password,
		Name:        req.GetName().Value,
	})
	if errors.Is(err, user.ErrAlreadyExists) {
		return &api.RegisterConflict{
			Title:  "User already exists",
			Status: http.StatusConflict,
			Detail: "User with that username already exists",
		}, nil
	} else if err != nil {
		slog.Error("error during user registration", slog.Any("error", err))

		return &api.RegisterInternalServerError{
			Title:  "Error registering user",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while registering user",
		}, nil
	}

	return &api.RegisterCreated{}, nil
}

// RefreshTokens generates a new pair of access and refresh tokens using previously issued refresh token.
func (h Handler) RefreshTokens(
	ctx context.Context,
	req *api.RefreshTokensReq,
) (api.RefreshTokensRes, error) {
	accessToken, refreshToken, err := h.uc.RefreshTokens(ctx, dto.TokensRefreshRequest{
		RequestTime:  time.Now().UTC(),
		RefreshToken: req.RefreshToken,
	})
	if errors.Is(err, user.ErrWrongTokenType) {
		return &api.RefreshTokensBadRequest{
			Title:  "Wrong token type",
			Status: http.StatusBadRequest,
			Detail: "The provided token is not a refresh token",
		}, nil
	} else if err != nil {
		slog.Error("error refreshing tokens", slog.Any("error", err))

		return &api.RefreshTokensInternalServerError{
			Title:  "Error refreshing tokens",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while refreshing tokens",
		}, nil
	}

	return &api.RefreshTokensNoContent{
		SetCookie: h.newCookieStringFromTokens(accessToken, refreshToken),
	}, nil
}

// GetUserSessions retrieves all active sessions associated with the authenticated user.
func (h Handler) GetUserSessions(ctx context.Context) (api.GetUserSessionsRes, error) {
	tokenClaims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.Error{
			Title:  "Error authorizing user",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	sessions, err := h.uc.GetSessions(ctx, tokenClaims.UserID)
	if err != nil {
		slog.Error("error getting user sessions", slog.Any("error", err))

		return &api.Error{
			Title:  "Error getting user sessions",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while getting user sessions",
		}, nil
	}

	return dto.SessionsToAPI(sessions), nil
}

// DeleteUserSession terminates a specific user session by its ID.
func (h Handler) DeleteUserSession(
	ctx context.Context,
	params api.DeleteUserSessionParams,
) (api.DeleteUserSessionRes, error) {
	tokenClaims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.Error{
			Title:  "Error authorizing user",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	if err := h.uc.DeleteSession(ctx, tokenClaims.UserID, params.ID); err != nil {
		slog.Error("error deleting user session", slog.Any("error", err))

		return &api.Error{
			Title:  "Error deleting user session",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while deleting user session",
		}, nil
	}

	return &api.DeleteUserSessionNoContent{}, nil
}

// GetOAuthProviders fetches all connected OAuth providers for the authenticated user.
func (h Handler) GetOAuthProviders(ctx context.Context) (api.GetOAuthProvidersRes, error) {
	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.GetOAuthProvidersUnauthorized{
			Title:  "Error authorizing user",
			Status: http.StatusUnauthorized,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	providers, err := h.uc.GetOAuth(ctx, claims.UserID)
	if err != nil {
		slog.Error("error getting oauth providers", slog.Any("error", err))

		return &api.GetOAuthProvidersInternalServerError{
			Title:  "Error getting oauth providers",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while getting oauth providers",
		}, nil
	}

	return dto.OAuthToAPI(providers), nil
}
