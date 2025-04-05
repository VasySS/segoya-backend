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

// NewYandex initiates the OAuth2 login process for Yandex to create a new oauth connection for the user.
func (h Handler) NewYandex(ctx context.Context) (api.NewYandexRes, error) {
	state := h.rnd.NewRandomHexString(h.cfg.oauthStateLen)
	cookieState := h.newOAuthCookieState(state)
	redirectURL := h.cfg.yandexNew.AuthCodeURL(state)

	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.Error{
			Title:  "Error authorizing user",
			Status: http.StatusUnauthorized,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	// save oauth state to get it user id by it in callback
	if err := h.uc.NewYandex(ctx, dto.NewOAuthRequest{
		RequestTime: time.Now().UTC(),
		StateTTL:    h.cfg.oauthCookieTTL,
		State:       state,
		UserID:      claims.UserID,
	}); err != nil {
		slog.Error("error creating new oauth connection", slog.Any("error", err))

		return &api.Error{
			Title:  "Error creating new oauth connection",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while creating new oauth connection",
		}, nil
	}

	return &api.NewYandexTemporaryRedirect{
		Location:  redirectURL,
		SetCookie: cookieState,
	}, nil
}

// NewYandexCallback handles the Yandex OAuth2 callback request after user authentication.
func (h Handler) NewYandexCallback(
	ctx context.Context,
	params api.NewYandexCallbackParams,
) (api.NewYandexCallbackRes, error) {
	codeParamsValue, err := url.QueryUnescape(params.Code)
	if err != nil || codeParamsValue == "" {
		return &api.NewYandexCallbackBadRequest{
			Title:  "Error parsing code",
			Status: http.StatusBadRequest,
			Detail: "An error occurred while parsing code",
		}, nil
	}

	stateParamsValue, err := url.QueryUnescape(params.State)
	if err != nil || stateParamsValue == "" {
		return &api.NewYandexCallbackBadRequest{
			Title:  "Error parsing state",
			Status: http.StatusBadRequest,
			Detail: "An error occurred while parsing state",
		}, nil
	}

	cookieState, err := h.parseCookieState(params.Cookie)
	if err != nil {
		return &api.NewYandexCallbackBadRequest{
			Title:  "Error parsing cookie",
			Status: http.StatusBadRequest,
			Detail: "An error occurred while parsing cookie header",
		}, nil
	}

	if stateParamsValue != cookieState {
		return &api.NewYandexCallbackInternalServerError{
			Title:  "Error parsing state",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while parsing state",
		}, nil
	}

	err = h.uc.NewYandexCallback(ctx, dto.NewOAuthCallbackRequest{
		RequestTime: time.Now().UTC(),
		Code:        codeParamsValue,
		State:       stateParamsValue,
	})
	if errors.Is(err, user.ErrOAuthAlreadyExists) {
		return &api.NewYandexCallbackBadRequest{
			Title:  "Error adding Yandex OAuth",
			Status: http.StatusBadRequest,
			Detail: "This Yandex account is already connected to another user",
		}, nil
	} else if err != nil {
		slog.Error("error adding yandex auth", slog.Any("error", err))

		return &api.NewYandexCallbackInternalServerError{
			Title:  "Error adding yandex auth",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while adding yandex auth",
		}, nil
	}

	return &api.NewYandexCallbackTemporaryRedirect{
		Location: h.cfg.frontendURL.String() + "/profile",
	}, nil
}

// YandexLogin initiates the OAuth2 login process for previously connected Yandex account.
func (h Handler) YandexLogin(_ context.Context) (*api.YandexLoginTemporaryRedirect, error) {
	state := h.rnd.NewRandomHexString(h.cfg.oauthStateLen)
	cookieState := h.newOAuthCookieState(state)
	redirectURL := h.cfg.yandexLogin.AuthCodeURL(state)

	return &api.YandexLoginTemporaryRedirect{
		Location:  redirectURL,
		SetCookie: cookieState,
	}, nil
}

// YandexLoginCallback handles the callback from the Yandex OAuth2 login page.
func (h Handler) YandexLoginCallback(
	ctx context.Context,
	params api.YandexLoginCallbackParams,
) (api.YandexLoginCallbackRes, error) {
	if params.Code == "" {
		return &api.YandexLoginCallbackBadRequest{
			Title:  "Error parsing code",
			Status: http.StatusBadRequest,
			Detail: "No code was found in request",
		}, nil
	}

	cookieState, err := h.parseCookieState(params.Cookie)
	if err != nil {
		return &api.YandexLoginCallbackBadRequest{
			Title:  "Error parsing cookie",
			Status: http.StatusBadRequest,
			Detail: "An error occurred while parsing cookie header",
		}, nil
	}

	if params.State != cookieState {
		return &api.YandexLoginCallbackBadRequest{
			Title:  "Error parsing state",
			Status: http.StatusBadRequest,
			Detail: "OAuth state does not match",
		}, nil
	}

	newTokensReq := dto.OAuthLoginCallbackRequest{
		RequestTime: time.Now().UTC(),
		Code:        params.Code,
	}

	accessToken, refreshToken, err := h.uc.LoginYandexCallback(ctx, newTokensReq)
	if errors.Is(err, user.ErrOAuthNotFound) {
		return &api.YandexLoginCallbackNotFound{
			Title:  "Error logging in with Yandex",
			Status: http.StatusNotFound,
			Detail: "No oauth connection was found",
		}, nil
	} else if err != nil {
		slog.Error("error logging in with yandex", slog.Any("error", err))

		return &api.YandexLoginCallbackInternalServerError{
			Title:  "Error logging in with yandex",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while logging in with yandex",
		}, nil
	}

	return &api.YandexLoginCallbackTemporaryRedirect{
		Location:  h.cfg.frontendURL.String() + "/profile",
		SetCookie: h.newCookieStringFromTokens(accessToken, refreshToken),
	}, nil
}

// DeleteYandex deletes OAuth2 Yandex connection for user.
func (h Handler) DeleteYandex(ctx context.Context) (api.DeleteYandexRes, error) {
	claims, ok := h.ts.FromContext(ctx)
	if !ok {
		return &api.DeleteYandexUnauthorized{
			Title:  "Error authorizing user",
			Status: http.StatusUnauthorized,
			Detail: "An error occurred while authorizing user",
		}, nil
	}

	if err := h.uc.DeleteYandex(ctx, dto.DeleteOAuthRequest{
		UserID: claims.UserID,
		Issuer: user.YandexOAuth,
	}); err != nil {
		slog.Error("error deleting yandex auth", slog.Any("error", err))

		return &api.DeleteYandexInternalServerError{
			Title:  "Error deleting yandex auth",
			Status: http.StatusInternalServerError,
			Detail: "An error occurred while deleting yandex auth",
		}, nil
	}

	return &api.DeleteYandexNoContent{}, nil
}
