package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/config"
	"github.com/VasySS/segoya-backend/internal/controller/http/v1/auth"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

type handlerTokens struct{ authorized bool }

func (s handlerTokens) FromContext(context.Context) (user.AccessTokenClaims, bool) {
	return user.AccessTokenClaims{UserID: authUserID()}, s.authorized
}

type acceptingCaptcha struct{}

func (acceptingCaptcha) IsTokenValid(context.Context, string) error { return nil }

type handlerUsecase struct {
	auth.Usecase

	t       *testing.T
	failure error
	calls   int
}

func (u *handlerUsecase) Login(ctx context.Context, req dto.LoginRequest) (string, string, error) {
	assert.NotZero(u.t, req.RequestTime)
	u.request(ctx, req.RequestTime)
	assert.Equal(u.t, "player", req.Username)
	assert.Equal(u.t, "password", req.Password)
	assert.Equal(u.t, "browser", req.UserAgent)

	return "access", "refresh", u.failure
}

func (u *handlerUsecase) Register(ctx context.Context, req dto.RegisterRequest) error {
	assert.NotZero(u.t, req.RequestTime)
	u.request(ctx, req.RequestTime)
	assert.Equal(u.t, "player", req.Username)
	assert.Equal(u.t, "password", req.Password)
	assert.Equal(u.t, "Player", req.Name)

	return u.failure
}

func (u *handlerUsecase) RefreshTokens(ctx context.Context, req dto.TokensRefreshRequest) (string, string, error) {
	assert.NotZero(u.t, req.RequestTime)
	u.request(ctx, req.RequestTime)
	assert.Equal(u.t, "old-refresh", req.RefreshToken)

	return "access", "refresh", u.failure
}

func (u *handlerUsecase) GetSessions(ctx context.Context, id uuid.UUID) ([]user.Session, error) {
	u.request(ctx, time.Time{})
	assert.Equal(u.t, authUserID(), id)

	return []user.Session{{UserID: authUserID(), SessionID: authSessionID(), UA: "browser"}}, u.failure
}

func (u *handlerUsecase) DeleteSession(ctx context.Context, uid, sid uuid.UUID) error {
	u.request(ctx, time.Time{})
	assert.Equal(u.t, authUserID(), uid)
	assert.Equal(u.t, authSessionID(), sid)

	return u.failure
}

func (u *handlerUsecase) GetOAuth(ctx context.Context, id uuid.UUID) ([]user.OAuth, error) {
	u.request(ctx, time.Time{})
	assert.Equal(u.t, authUserID(), id)

	return []user.OAuth{{Issuer: user.YandexOAuth}}, u.failure
}

func newHandlerConfig() auth.Config {
	var cfg config.Config

	cfg.ENV.FrontendURL = url.URL{Scheme: "https", Host: "frontend.example"}
	cfg.Limits.AccessTokenTTL = time.Minute
	cfg.Limits.RefreshTokenTTL = time.Hour

	return auth.NewConfig(cfg)
}

func assertTokenCookies(t *testing.T, result any) {
	t.Helper()

	var raw string

	switch response := result.(type) {
	case *api.LoginNoContent:
		raw = response.SetCookie
	case *api.RefreshTokensNoContent:
		raw = response.SetCookie
	case *api.DiscordLoginCallbackTemporaryRedirect:
		raw = response.SetCookie
	case *api.YandexLoginCallbackTemporaryRedirect:
		raw = response.SetCookie
	default:
		t.Fatalf("expected token cookies, got %T", result)
	}

	// Split only to inspect each logical cookie. Real HTTP header framing remains
	// a separate production follow-up; a single combined value is not valid.
	response := &http.Response{Header: http.Header{"Set-Cookie": strings.Split(raw, ",")}}
	cookies := response.Cookies()
	require.Len(t, cookies, 2)

	wantTokens := map[string]string{"accessToken": "access", "refreshToken": "refresh"}
	wantAge := map[string]int{"accessToken": 60, "refreshToken": 3600}

	for _, cookie := range cookies {
		require.Contains(t, wantTokens, cookie.Name)
		assert.Equal(t, wantTokens[cookie.Name], cookie.Value)
		assert.Equal(t, wantAge[cookie.Name], cookie.MaxAge)
		assert.True(t, cookie.Secure, cookie.Name)
		assert.True(t, cookie.HttpOnly, cookie.Name)
		assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite, cookie.Name)
		assert.Equal(t, "/", cookie.Path)
		assert.Equal(t, "frontend.example", cookie.Domain)
		delete(wantTokens, cookie.Name)
	}

	assert.Empty(t, wantTokens)
}

func TestDefaultHandlerContracts(t *testing.T) {
	t.Parallel()

	type rejection struct {
		name   string
		cause  error
		want   any
		status int
	}

	for _, operation := range []struct {
		name                           string
		invoke                         func(context.Context, *auth.Handler) (any, error)
		want, unauthorized, unexpected any
		cookies                        bool
		rejections                     []rejection
	}{
		{
			name: "login", invoke: func(ctx context.Context, h *auth.Handler) (any, error) {
				return h.Login(ctx,
					&api.LoginRequest{
						Username: "player",
						Password: "password",
					},
					api.LoginParams{UserAgent: "browser"})
			},
			want: &api.LoginNoContent{}, unexpected: &api.LoginInternalServerError{}, cookies: true,
			rejections: []rejection{
				{"unknown user", user.ErrUserNotFound, &api.LoginUnauthorized{}, 401},
				{"wrong password", user.ErrWrongPassword, &api.LoginUnauthorized{}, 401},
			},
		},
		{
			name: "register", invoke: func(ctx context.Context, h *auth.Handler) (any, error) {
				return h.Register(ctx,
					&api.RegisterRequest{
						Username: "player",
						Password: "password",
						Name:     api.NewOptString("Player"),
					},
					api.RegisterParams{})
			},
			want: &api.RegisterCreated{}, unexpected: &api.RegisterInternalServerError{},
			rejections: []rejection{{"duplicate user", user.ErrAlreadyExists, &api.RegisterConflict{}, 409}},
		},
		{
			name: "refresh", invoke: func(ctx context.Context, h *auth.Handler) (any, error) {
				return h.RefreshTokens(ctx, &api.RefreshTokensReq{RefreshToken: "old-refresh"})
			},
			want: &api.RefreshTokensNoContent{}, unexpected: &api.RefreshTokensInternalServerError{}, cookies: true,
			rejections: []rejection{{"wrong token type", user.ErrWrongTokenType, &api.RefreshTokensBadRequest{}, 400}},
		},
		{
			name: "sessions", invoke: func(ctx context.Context, h *auth.Handler) (any, error) { return h.GetUserSessions(ctx) },
			want: &api.GetUserSessionsOKApplicationJSON{{
				UserID:    authUserID(),
				SessionID: authSessionID(),
				Ua:        "browser",
			}},

			unauthorized: &api.BackendError{}, unexpected: &api.BackendError{},
		},
		{
			name: "delete session", invoke: func(ctx context.Context, h *auth.Handler) (any, error) {
				return h.DeleteUserSession(ctx, api.DeleteUserSessionParams{ID: authSessionID()})
			},
			want: &api.DeleteUserSessionNoContent{}, unauthorized: &api.BackendError{}, unexpected: &api.BackendError{},
		},
		{
			name: "providers",
			invoke: func(ctx context.Context,
				h *auth.Handler) (any,
				error,
			) {
				return h.GetOAuthProviders(ctx)
			},

			want:         &api.GetOAuthProvidersOKApplicationJSON{{Provider: "yandex"}},
			unauthorized: &api.GetOAuthProvidersUnauthorized{},
			unexpected:   &api.GetOAuthProvidersInternalServerError{},
		},
	} {
		t.Run(operation.name, func(t *testing.T) {
			t.Parallel()

			cases := append([]rejection{
				{
					name: "success",
					want: operation.want,
				},
				{
					"unexpected failure",
					errors.New("private database detail"),
					operation.unexpected,
					500,
				},
			},
				operation.rejections...)
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()

					uc := &handlerUsecase{t: t}
					if tc.cause != nil {
						uc.failure = fmt.Errorf("wrapped: %w", tc.cause)
					}

					h := auth.NewHandler(newHandlerConfig(), uc, nil, handlerTokens{authorized: true}, acceptingCaptcha{})
					ctx := context.WithValue(t.Context(), requestContextKey{}, "metadata")
					result, err := operation.invoke(ctx, h)
					require.NoError(t, err)
					assert.Equal(t, 1, uc.calls)
					assert.IsType(t, tc.want, result)

					if tc.cause == nil {
						if operation.cookies {
							assertTokenCookies(t, result)
						} else {
							assert.Equal(t, tc.want, result)
						}

						return
					}

					body, err := json.Marshal(result)
					require.NoError(t, err)

					var problem api.BackendError

					require.NoError(t, json.Unmarshal(body, &problem))
					assert.Equal(t, tc.status, problem.Status)
					assert.NotEmpty(t, problem.Title)
					assert.NotEmpty(t, problem.Detail)
					assert.NotContains(t, problem.Detail, "private database detail")
				})
			}

			if operation.unauthorized != nil {
				t.Run("unauthorized", func(t *testing.T) {
					t.Parallel()
					uc := &handlerUsecase{t: t}
					result,
						err := operation.invoke(t.Context(),
						auth.NewHandler(newHandlerConfig(),
							uc,
							nil,
							handlerTokens{},
							acceptingCaptcha{}))
					require.NoError(t, err)
					assert.IsType(t, operation.unauthorized, result)
					assert.Zero(t, uc.calls)
				})
			}
		})
	}
}

func authUserID() uuid.UUID { return uuid.MustParse("00000000-0000-0000-0000-000000000001") }

func authSessionID() uuid.UUID { return uuid.MustParse("00000000-0000-0000-0000-000000000002") }

func (u *handlerUsecase) request(ctx context.Context, at time.Time) {
	u.t.Helper()

	u.calls++
	assert.Equal(u.t, "metadata", ctx.Value(requestContextKey{}))

	if !at.IsZero() {
		assert.WithinDuration(u.t, time.Now().UTC(), at, time.Second)
	}
}
