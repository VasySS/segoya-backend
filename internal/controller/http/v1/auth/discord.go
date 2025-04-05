package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// NewDiscord initiates the OAuth2 login process for Discord to create a new oauth connection for the user.
func (h Handler) NewDiscord(ctx context.Context) (api.NewDiscordRes, error) {
	state := h.rnd.NewRandomHexString(h.cfg.oauthStateLen)
	cookieState := h.newOAuthCookieState(state)
	redirectURL := h.cfg.discordNew.AuthCodeURL(state)

	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.Error{
			Title:  "Error authorizing user",
			Status: http.StatusUnauthorized,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	err := h.uc.NewDiscord(ctx, dto.NewOAuthRequest{
		RequestTime: time.Now().UTC(),
		StateTTL:    h.cfg.oauthCookieTTL,
		State:       state,
		UserID:      claims.UserID,
	})
	if err != nil {
		slog.Error("error creating new oauth connection", slog.Any("error", err))

		return &api.Error{
			Title:  "Error creating new oauth connection",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while creating new oauth connection",
		}, nil
	}

	return &api.NewDiscordTemporaryRedirect{
		Location:  redirectURL,
		SetCookie: cookieState,
	}, nil
}

// NewDiscordCallback handles the Discord OAuth2 callback request after user authentication.
func (h Handler) NewDiscordCallback(
	ctx context.Context,
	params api.NewDiscordCallbackParams,
) (api.NewDiscordCallbackRes, error) {
	codeParamsValue, err := url.QueryUnescape(params.Code)
	if err != nil || params.Code == "" {
		return &api.NewDiscordCallbackBadRequest{
			Title:  "Error parsing code",
			Status: http.StatusBadRequest,
			Detail: "No code was found in request",
		}, nil
	}

	stateParamsValue, err := url.QueryUnescape(params.State)
	if err != nil || params.State == "" {
		return &api.NewDiscordCallbackBadRequest{
			Title:  "Error parsing state",
			Status: http.StatusBadRequest,
			Detail: "No state was found in request",
		}, nil
	}

	cookieState, err := h.parseCookieState(params.Cookie)
	if err != nil {
		return &api.NewDiscordCallbackBadRequest{
			Title:  "Error parsing cookie",
			Status: http.StatusBadRequest,
			Detail: "An error occurred while parsing cookie header",
		}, nil
	}

	if stateParamsValue != cookieState {
		return &api.NewDiscordCallbackBadRequest{
			Title:  "Error parsing state",
			Status: http.StatusBadRequest,
			Detail: "State in request does not match state in cookie",
		}, nil
	}

	err = h.uc.NewDiscordCallback(ctx, dto.NewOAuthCallbackRequest{
		RequestTime: time.Now().UTC(),
		Code:        codeParamsValue,
		State:       stateParamsValue,
	})
	if errors.Is(err, user.ErrOAuthAlreadyExists) {
		return &api.NewDiscordCallbackBadRequest{
			Title:  "Error adding Discord auth",
			Status: http.StatusBadRequest,
			Detail: "This Discord account is already connected to another user",
		}, nil
	} else if err != nil {
		slog.Error("error adding discord auth", slog.Any("error", err))

		return &api.NewDiscordCallbackInternalServerError{
			Title:  "Error adding Discord auth",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while adding Discord auth",
		}, nil
	}

	return &api.NewDiscordCallbackTemporaryRedirect{
		Location: h.cfg.frontendURL.String() + "/profile",
	}, nil
}

// DiscordLogin initiates the OAuth2 login process for previously connected Discord account.
func (h Handler) DiscordLogin(_ context.Context) (*api.DiscordLoginTemporaryRedirect, error) {
	state := h.rnd.NewRandomHexString(h.cfg.oauthStateLen)
	cookieState := h.newOAuthCookieState(state)
	redirectURL := h.cfg.discordLogin.AuthCodeURL(state)

	return &api.DiscordLoginTemporaryRedirect{
		Location:  redirectURL,
		SetCookie: cookieState,
	}, nil
}

// DiscordLoginCallback handles the callback from the Discord OAuth2 login page.
func (h Handler) DiscordLoginCallback(
	ctx context.Context,
	params api.DiscordLoginCallbackParams,
) (api.DiscordLoginCallbackRes, error) {
	if params.Code == "" {
		return &api.DiscordLoginCallbackBadRequest{
			Title:  "Error parsing code",
			Status: http.StatusBadRequest,
			Detail: "No code was found in request",
		}, nil
	}

	cookieState, err := h.parseCookieState(params.Cookie)
	if err != nil {
		return &api.DiscordLoginCallbackBadRequest{
			Title:  "Error parsing cookie",
			Status: http.StatusBadRequest,
			Detail: "An error occurred while parsing cookie header",
		}, nil
	}

	if params.State != cookieState {
		return &api.DiscordLoginCallbackBadRequest{
			Title:  "Error parsing state",
			Status: http.StatusBadRequest,
			Detail: "State in request does not match state in cookie",
		}, nil
	}

	oauthLoginReq := dto.OAuthLoginCallbackRequest{
		RequestTime: time.Now().UTC(),
		Code:        params.Code,
	}

	accessToken, refreshToken, err := h.uc.LoginDiscordCallback(ctx, oauthLoginReq)
	if errors.Is(err, user.ErrOAuthNotFound) {
		return &api.DiscordLoginCallbackNotFound{
			Title:  "Error logging in with Discord",
			Status: http.StatusNotFound,
			Detail: "No oauth connection was found",
		}, nil
	} else if err != nil {
		slog.Error("error logging in with discord", slog.Any("error", err))

		return &api.DiscordLoginCallbackInternalServerError{
			Title:  "Error logging in with discord",
			Status: http.StatusForbidden,
			Detail: "An error occurred while logging in with discord",
		}, nil
	}

	return &api.DiscordLoginCallbackTemporaryRedirect{
		Location:  h.cfg.frontendURL.String() + "/profile",
		SetCookie: h.newCookieStringFromTokens(accessToken, refreshToken),
	}, nil
}

// DeleteDiscord deletes OAuth2 Discord connection for user.
func (h Handler) DeleteDiscord(ctx context.Context) (api.DeleteDiscordRes, error) {
	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.DeleteDiscordUnauthorized{
			Title:  "Error authorizing user",
			Status: http.StatusUnauthorized,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	if err := h.uc.DeleteDiscord(ctx, dto.DeleteOAuthRequest{
		UserID: claims.UserID,
		Issuer: user.DiscordOAuth,
	}); err != nil {
		slog.Error("error deleting discord auth", slog.Any("error", err))

		return &api.DeleteDiscordInternalServerError{
			Title:  "Error deleting discord auth",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while deleting discord auth",
		}, nil
	}

	return &api.DeleteDiscordNoContent{}, nil
}
