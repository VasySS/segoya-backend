package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/usecase/auth"
)

type oauthProvider struct {
	issuer      user.OAuthIssuer
	exchange    string
	linkConfig  func(auth.Config) oauth2.Config
	loginConfig func(auth.Config) oauth2.Config
	begin       func(*auth.Usecase, context.Context, dto.NewOAuthRequest) error
	link        func(*auth.Usecase, context.Context, dto.NewOAuthCallbackRequest) error
	login       func(*auth.Usecase, context.Context, dto.OAuthLoginCallbackRequest) (string, string, error)
	remove      func(*auth.Usecase, context.Context, dto.DeleteOAuthRequest) error
}

func TestUsecase_DiscordOAuth(t *testing.T) {
	t.Parallel()
	testOAuthProvider(t, oauthProvider{
		issuer: user.DiscordOAuth, exchange: "ExchangeDiscordCodeForID",
		linkConfig:  func(c auth.Config) oauth2.Config { return c.DiscordNew },
		loginConfig: func(c auth.Config) oauth2.Config { return c.DiscordLogin },
		begin:       (*auth.Usecase).NewDiscord, link: (*auth.Usecase).NewDiscordCallback,
		login: (*auth.Usecase).LoginDiscordCallback, remove: (*auth.Usecase).DeleteDiscord,
	})
}

func testOAuthProvider(t *testing.T, provider oauthProvider) {
	t.Helper()
	t.Run("begin linking", func(t *testing.T) { testOAuthBegin(t, provider) })
	t.Run("link callback", func(t *testing.T) { testOAuthLink(t, provider) })
	t.Run("login callback", func(t *testing.T) { testOAuthLogin(t, provider) })
	t.Run("delete connection", func(t *testing.T) { testOAuthDelete(t, provider) })
}

func testOAuthBegin(t *testing.T, provider oauthProvider) {
	t.Helper()
	t.Parallel()

	for _, scenario := range []string{"success", "state persistence failure"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			h := newAuthHarness(t)
			req := dto.NewOAuthRequest{
				RequestTime: h.now,
				StateTTL:    5 * time.Minute,
				State:       "random-state",
				UserID:      h.profile.ID,
			}

			var want error
			if scenario != "success" {
				want = errors.New("state unavailable")
			}

			h.sessions.On("NewOAuthState", mock.Anything, req).Return(want).Once()
			require.ErrorIs(t, provider.begin(h.uc, t.Context(), req), want)
		})
	}
}

func testOAuthLink(t *testing.T, provider oauthProvider) {
	t.Helper()
	t.Parallel()

	for _, failure := range []string{"", "state", "exchange", "insert"} {
		name := failure + " failure"
		if failure == "" {
			name = "success"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newAuthHarness(t)
			cause := errors.New("oauth dependency unavailable")
			req := dto.NewOAuthCallbackRequest{RequestTime: h.now, State: "saved-state", Code: "link-code"}
			setup := func() {
				if failure == "state" {
					h.sessions.On("GetOAuthUserID", mock.Anything, req.State).Return(h.profile.ID, cause).Once()
					return
				}

				state := h.sessions.On("GetOAuthUserID", mock.Anything, req.State).Return(h.profile.ID, nil).Once()

				if failure == "exchange" {
					exchange := h.tokens.On(provider.exchange, mock.Anything, provider.linkConfig(h.config), req.Code).
						Return("", cause).Once()
					mock.InOrder(state, exchange)

					return
				}

				exchange := h.tokens.On(provider.exchange, mock.Anything, provider.linkConfig(h.config), req.Code).
					Return("provider-user-id", nil).Once()

				var insertErr error
				if failure == "insert" {
					insertErr = cause
				}

				insert := h.users.On("NewOAuth", mock.Anything, dto.NewOAuthRequestDB{
					RequestTime: h.now, OAuthID: "provider-user-id", UserID: h.profile.ID, Issuer: provider.issuer,
				}).Return(insertErr).Once()
				mock.InOrder(state, exchange, insert)
			}
			setup()

			var want error
			if failure != "" {
				want = cause
			}

			require.ErrorIs(t, provider.link(h.uc, t.Context(), req), want)
		})
	}
}

func testOAuthLogin(t *testing.T, provider oauthProvider) {
	t.Helper()
	t.Parallel()

	for _, failure := range []string{"", "exchange", "lookup", "access token", "refresh token"} {
		name := failure + " failure"
		if failure == "" {
			name = "success"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newAuthHarness(t)
			cause := errors.New("oauth dependency unavailable")
			req := dto.OAuthLoginCallbackRequest{RequestTime: h.now, Code: "login-code"}
			setup := func() {
				if failure == "exchange" {
					h.tokens.On(provider.exchange, mock.Anything, provider.loginConfig(h.config), req.Code).Return("", cause).Once()
					return
				}

				exchange := h.tokens.On(provider.exchange, mock.Anything, provider.loginConfig(h.config), req.Code).
					Return("provider-user-id", nil).Once()

				lookupReq := dto.GetUserByOAuthRequest{OAuthID: "provider-user-id", Issuer: provider.issuer}
				if failure == "lookup" {
					lookup := h.users.On("GetUserByOAuth", mock.Anything, lookupReq).Return(user.PrivateProfile{}, cause).Once()
					mock.InOrder(exchange, lookup)

					return
				}

				lookup := h.users.On("GetUserByOAuth", mock.Anything, lookupReq).Return(h.profile, nil).Once()
				id := h.crypto.On("NewUUID7").Return(h.session).Once()
				mock.InOrder(exchange, lookup, id)

				if failure == "access token" {
					h.tokens.On("NewAccessToken", h.now, h.accessClaims()).Return("", cause).Once()
					return
				}

				h.tokens.On("NewAccessToken", h.now, h.accessClaims()).Return("access", nil).Once()

				var tokenErr error
				if failure == "refresh token" {
					tokenErr = cause
				}

				h.tokens.On("NewRefreshToken", h.now, h.refreshClaims()).Return("refresh", tokenErr).Once()
			}
			setup()

			access, refresh, err := provider.login(h.uc, t.Context(), req)
			if failure != "" {
				require.ErrorIs(t, err, cause)
				assert.Empty(t, access)
				assert.Empty(t, refresh)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, "access", access)
			assert.Equal(t, "refresh", refresh)
		})
	}
}

func testOAuthDelete(t *testing.T, provider oauthProvider) {
	t.Helper()
	t.Parallel()

	for _, scenario := range []string{"success", "delete failure"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			h := newAuthHarness(t)
			req := dto.DeleteOAuthRequest{UserID: h.profile.ID, Issuer: provider.issuer}

			var want error
			if scenario != "success" {
				want = errors.New("delete unavailable")
			}

			h.users.On("DeleteOAuth", mock.Anything, req).Return(want).Once()
			require.ErrorIs(t, provider.remove(h.uc, t.Context(), req), want)
		})
	}
}
