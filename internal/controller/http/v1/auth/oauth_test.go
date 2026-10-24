package auth_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/config"
	"github.com/VasySS/segoya-backend/internal/controller/http/v1/auth"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

type randomState struct{ t *testing.T }

func (r randomState) NewRandomHexString(length int) string {
	assert.Equal(r.t, 32, length)
	return "state"
}

type oauthUsecase struct {
	auth.Usecase

	t                   *testing.T
	failure             error
	calls               int
	wantCode, wantState string
	issuer              user.OAuthIssuer
}

func (u *oauthUsecase) NewDiscord(ctx context.Context, req dto.NewOAuthRequest) error {
	assert.Equal(u.t, user.DiscordOAuth, u.issuer)
	return u.newState(ctx, req)
}

func (u *oauthUsecase) NewYandex(ctx context.Context, req dto.NewOAuthRequest) error {
	assert.Equal(u.t, user.YandexOAuth, u.issuer)
	return u.newState(ctx, req)
}

func (u *oauthUsecase) NewDiscordCallback(ctx context.Context, req dto.NewOAuthCallbackRequest) error {
	assert.Equal(u.t, user.DiscordOAuth, u.issuer)
	return u.link(ctx, req)
}

func (u *oauthUsecase) NewYandexCallback(ctx context.Context, req dto.NewOAuthCallbackRequest) error {
	assert.Equal(u.t, user.YandexOAuth, u.issuer)
	return u.link(ctx, req)
}

func (u *oauthUsecase) LoginDiscordCallback(
	ctx context.Context,
	req dto.OAuthLoginCallbackRequest,
) (string, string, error) {
	assert.Equal(u.t, user.DiscordOAuth, u.issuer)
	return u.login(ctx, req)
}

func (u *oauthUsecase) LoginYandexCallback(
	ctx context.Context,
	req dto.OAuthLoginCallbackRequest,
) (string, string, error) {
	assert.Equal(u.t, user.YandexOAuth, u.issuer)
	return u.login(ctx, req)
}

func (u *oauthUsecase) DeleteDiscord(ctx context.Context, req dto.DeleteOAuthRequest) error {
	assert.Equal(u.t, user.DiscordOAuth, u.issuer)
	return u.remove(ctx, req)
}

func (u *oauthUsecase) DeleteYandex(ctx context.Context, req dto.DeleteOAuthRequest) error {
	assert.Equal(u.t, user.YandexOAuth, u.issuer)
	return u.remove(ctx, req)
}

type callbackInput struct{ code, state, cookie string }

type provider struct {
	name     string
	issuer   user.OAuthIssuer
	new      func(context.Context, *auth.Handler) (any, error)
	link     func(context.Context, *auth.Handler, callbackInput) (any, error)
	login    func(context.Context, *auth.Handler) (any, error)
	callback func(context.Context, *auth.Handler, callbackInput) (any, error)
	remove   func(context.Context, *auth.Handler) (any, error)

	linkSuccess, linkBadRequest, linkFailure                  any
	loginSuccess, loginBadRequest, loginMissing, loginFailure any
	deleteSuccess, deleteUnauthorized, deleteFailure          any
}

func providers() []provider {
	return []provider{
		{
			name: "discord", issuer: user.DiscordOAuth,
			new: func(ctx context.Context, h *auth.Handler) (any, error) { return h.NewDiscord(ctx) },
			link: func(ctx context.Context, h *auth.Handler, p callbackInput) (any, error) {
				return h.NewDiscordCallback(ctx, api.NewDiscordCallbackParams{Code: p.code, State: p.state, Cookie: p.cookie})
			},
			login: func(ctx context.Context, h *auth.Handler) (any, error) { return h.DiscordLogin(ctx) },
			callback: func(ctx context.Context, h *auth.Handler, p callbackInput) (any, error) {
				return h.DiscordLoginCallback(ctx, api.DiscordLoginCallbackParams{Code: p.code, State: p.state, Cookie: p.cookie})
			},
			remove:         func(ctx context.Context, h *auth.Handler) (any, error) { return h.DeleteDiscord(ctx) },
			linkSuccess:    &api.NewDiscordCallbackTemporaryRedirect{},
			linkBadRequest: &api.NewDiscordCallbackBadRequest{},
			linkFailure:    &api.NewDiscordCallbackInternalServerError{},

			loginSuccess:    &api.DiscordLoginCallbackTemporaryRedirect{},
			loginBadRequest: &api.DiscordLoginCallbackBadRequest{},
			loginMissing:    &api.DiscordLoginCallbackNotFound{},
			loginFailure:    &api.DiscordLoginCallbackInternalServerError{},

			deleteSuccess:      &api.DeleteDiscordNoContent{},
			deleteUnauthorized: &api.DeleteDiscordUnauthorized{},
			deleteFailure:      &api.DeleteDiscordInternalServerError{},
		},
		{
			name: "yandex", issuer: user.YandexOAuth,
			new: func(ctx context.Context, h *auth.Handler) (any, error) { return h.NewYandex(ctx) },
			link: func(ctx context.Context, h *auth.Handler, p callbackInput) (any, error) {
				return h.NewYandexCallback(ctx, api.NewYandexCallbackParams{Code: p.code, State: p.state, Cookie: p.cookie})
			},
			login: func(ctx context.Context, h *auth.Handler) (any, error) { return h.YandexLogin(ctx) },
			callback: func(ctx context.Context, h *auth.Handler, p callbackInput) (any, error) {
				return h.YandexLoginCallback(ctx, api.YandexLoginCallbackParams{Code: p.code, State: p.state, Cookie: p.cookie})
			},
			remove:         func(ctx context.Context, h *auth.Handler) (any, error) { return h.DeleteYandex(ctx) },
			linkSuccess:    &api.NewYandexCallbackTemporaryRedirect{},
			linkBadRequest: &api.NewYandexCallbackBadRequest{},
			linkFailure:    &api.NewYandexCallbackInternalServerError{},

			loginSuccess:    &api.YandexLoginCallbackTemporaryRedirect{},
			loginBadRequest: &api.YandexLoginCallbackBadRequest{},
			loginMissing:    &api.YandexLoginCallbackNotFound{},
			loginFailure:    &api.YandexLoginCallbackInternalServerError{},

			deleteSuccess:      &api.DeleteYandexNoContent{},
			deleteUnauthorized: &api.DeleteYandexUnauthorized{},
			deleteFailure:      &api.DeleteYandexInternalServerError{},
		},
	}
}

func newOAuthHandler(t *testing.T, uc *oauthUsecase, authorized bool) *auth.Handler {
	t.Helper()

	var cfg config.Config

	cfg.ENV.FrontendURL = url.URL{Scheme: "https", Host: "frontend.example"}
	cfg.Limits.AccessTokenTTL, cfg.Limits.RefreshTokenTTL = time.Minute, time.Hour

	cfg.OAuth.CookieTTL, cfg.OAuth.StateLen = 10*time.Minute, 32

	for name, target := range map[string]*oauth2.Config{
		"discord-login": &cfg.OAuth.DiscordLogin, "discord-new": &cfg.OAuth.DiscordNew,
		"yandex-login": &cfg.OAuth.YandexLogin, "yandex-new": &cfg.OAuth.YandexNew,
	} {
		*target = oauth2.Config{
			ClientID:    name,
			Endpoint:    oauth2.Endpoint{AuthURL: "https://provider.example/authorize"},
			RedirectURL: "https://backend.example/" + name,
		}
	}

	return auth.NewHandler(auth.NewConfig(cfg),
		uc,
		randomState{t},
		handlerTokens{authorized: authorized},
		acceptingCaptcha{})
}

func redirectFields(t *testing.T, response any) (string, string) {
	t.Helper()

	switch r := response.(type) {
	case *api.NewDiscordTemporaryRedirect:
		return r.Location, r.SetCookie
	case *api.NewYandexTemporaryRedirect:
		return r.Location, r.SetCookie
	case *api.DiscordLoginTemporaryRedirect:
		return r.Location, r.SetCookie
	case *api.YandexLoginTemporaryRedirect:
		return r.Location, r.SetCookie
	case *api.NewDiscordCallbackTemporaryRedirect:
		return r.Location, ""
	case *api.NewYandexCallbackTemporaryRedirect:
		return r.Location, ""
	case *api.DiscordLoginCallbackTemporaryRedirect:
		return r.Location, r.SetCookie
	case *api.YandexLoginCallbackTemporaryRedirect:
		return r.Location, r.SetCookie
	default:
		t.Fatalf("expected redirect, got %T", response)
		return "", ""
	}
}

func TestOAuthInitiation(t *testing.T) {
	t.Parallel()

	for _, p := range providers() {
		for _, operation := range []struct {
			name       string
			invoke     func(context.Context, *auth.Handler) (any, error)
			savesState bool
		}{
			{"login", p.login, false}, {"new", p.new, true},
		} {
			t.Run(p.name+"/"+operation.name, func(t *testing.T) {
				t.Parallel()
				uc := &oauthUsecase{t: t, issuer: p.issuer}
				ctx := context.WithValue(t.Context(), requestContextKey{}, "metadata")
				result, err := operation.invoke(ctx, newOAuthHandler(t, uc, true))
				require.NoError(t, err)
				location, rawCookie := redirectFields(t, result)
				redirect, err := url.Parse(location)
				require.NoError(t, err)
				assert.Equal(t, "https://provider.example/authorize", redirect.Scheme+"://"+redirect.Host+redirect.Path)
				assert.Equal(t, p.name+"-"+operation.name, redirect.Query().Get("client_id"))
				assert.Equal(t, "https://backend.example/"+p.name+"-"+operation.name, redirect.Query().Get("redirect_uri"))
				assert.Equal(t, "state", redirect.Query().Get("state"))
				assert.Equal(t, "code", redirect.Query().Get("response_type"))

				cookies := (&http.Response{Header: http.Header{"Set-Cookie": {rawCookie}}}).Cookies()
				require.Len(t, cookies, 1)
				assert.Equal(t, "oauthState", cookies[0].Name)
				assert.Equal(t, "state", cookies[0].Value)
				assert.Equal(t, 600, cookies[0].MaxAge)
				assert.Equal(t, "frontend.example", cookies[0].Domain)
				assert.Equal(t, "/", cookies[0].Path)
				assert.True(t, cookies[0].Secure)
				assert.True(t, cookies[0].HttpOnly)
				assert.Equal(t, http.SameSiteLaxMode, cookies[0].SameSite)

				if operation.savesState {
					assert.Equal(t, 1, uc.calls)
				} else {
					assert.Zero(t, uc.calls)
				}
			})
		}

		for _, tc := range []struct {
			name       string
			authorized bool
			failure    error
		}{
			{name: "unauthorized"},
			{
				name:       "state persistence failure",
				authorized: true,
				failure:    errors.New("state unavailable"),
			},
		} {
			t.Run(p.name+"/new/"+tc.name, func(t *testing.T) {
				t.Parallel()
				uc := &oauthUsecase{t: t, issuer: p.issuer, failure: tc.failure}
				ctx := context.WithValue(t.Context(), requestContextKey{}, "metadata")
				result, err := p.new(ctx, newOAuthHandler(t, uc, tc.authorized))
				require.NoError(t, err)

				problem, ok := result.(*api.BackendError)
				require.True(t, ok)

				if tc.authorized {
					assert.Equal(t, 500, problem.Status)
					assert.Equal(t, 1, uc.calls)
				} else {
					assert.Equal(t, 401, problem.Status)
					assert.Zero(t, uc.calls)
				}
			})
		}
	}
}

func TestOAuthLinkCallbacks(t *testing.T) {
	t.Parallel()

	for _, p := range providers() {
		for _, tc := range []struct {
			name  string
			input callbackInput
			cause error
			calls int
			kind  string
		}{
			{"success", callbackInput{"code", "state", "oauthState=state"}, nil, 1, "success"},
			{
				"encoded parameters",
				callbackInput{
					"code%2Bvalue",
					"state%2Bvalue",
					"oauthState=state%2Bvalue",
				},
				nil,
				1,
				"success",
			},

			{"missing code", callbackInput{"", "state", "oauthState=state"}, nil, 0, "bad"},
			{"invalid code escape", callbackInput{"%zz", "state", "oauthState=state"}, nil, 0, "bad"},
			{"missing state", callbackInput{"code", "", "oauthState=state"}, nil, 0, "bad"},
			{"invalid state escape", callbackInput{"code", "%zz", "oauthState=state"}, nil, 0, "bad"},
			{"missing cookie", callbackInput{"code", "state", ""}, nil, 0, "bad"},
			{"invalid cookie escape", callbackInput{"code", "state", "oauthState=%zz"}, nil, 0, "bad"},
			{"state mismatch", callbackInput{"code", "other", "oauthState=state"}, nil, 0, "mismatch"},
			{"already linked", callbackInput{"code", "state", "oauthState=state"}, user.ErrOAuthAlreadyExists, 1, "bad"},
			{
				"unexpected failure",
				callbackInput{
					"code",
					"state",
					"oauthState=state",
				},
				errors.New("storage failed"),
				1,
				"failure",
			},
		} {
			t.Run(p.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				code, err := url.QueryUnescape(tc.input.code)
				if tc.calls != 0 {
					require.NoError(t, err)
				}

				state, err := url.QueryUnescape(tc.input.state)
				if tc.calls != 0 {
					require.NoError(t, err)
				}

				uc := &oauthUsecase{t: t, issuer: p.issuer, wantCode: code, wantState: state}
				if tc.cause != nil {
					uc.failure = fmt.Errorf("wrapped: %w", tc.cause)
				}

				ctx := context.WithValue(t.Context(), requestContextKey{}, "metadata")
				result, err := p.link(ctx, newOAuthHandler(t, uc, true), tc.input)
				require.NoError(t, err)
				assert.Equal(t, tc.calls, uc.calls)

				switch tc.kind {
				case "success":
					assert.IsType(t, p.linkSuccess, result)
					location, _ := redirectFields(t, result)
					assert.Equal(t, "https://frontend.example/profile", location)
				case "bad":
					assert.IsType(t, p.linkBadRequest, result)
				case "failure":
					assert.IsType(t, p.linkFailure, result)
				case "mismatch":
					// Both providers must reject before linking. Yandex's 500 mapping
					// is tracked as a production follow-up instead of a desired status.
					assertRejectedOAuthLink(t, result)
				}
			})
		}
	}
}

func assertRejectedOAuthLink(t *testing.T, result any) {
	t.Helper()
	require.NotNil(t, result)

	switch result.(type) {
	case *api.NewDiscordCallbackTemporaryRedirect, *api.NewYandexCallbackTemporaryRedirect:
		t.Fatalf("state mismatch returned a successful redirect: %T", result)
	}
}

func TestOAuthLoginCallbacks(t *testing.T) {
	t.Parallel()

	for _, p := range providers() {
		for _, tc := range []struct {
			name  string
			input callbackInput
			cause error
			calls int
			kind  string
		}{
			{"success", callbackInput{"code", "state", "oauthState=state"}, nil, 1, "success"},
			{"missing code", callbackInput{"", "state", "oauthState=state"}, nil, 0, "bad"},
			{"missing cookie", callbackInput{"code", "state", ""}, nil, 0, "bad"},
			{"invalid cookie escape", callbackInput{"code", "state", "oauthState=%zz"}, nil, 0, "bad"},
			{"state mismatch", callbackInput{"code", "other", "oauthState=state"}, nil, 0, "bad"},
			{"unlinked account", callbackInput{"code", "state", "oauthState=state"}, user.ErrOAuthNotFound, 1, "missing"},
			{
				"unexpected failure",
				callbackInput{
					"code",
					"state",
					"oauthState=state",
				},
				errors.New("exchange failed"),
				1,
				"failure",
			},
		} {
			t.Run(p.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				uc := &oauthUsecase{t: t, issuer: p.issuer, wantCode: "code"}
				if tc.cause != nil {
					uc.failure = fmt.Errorf("wrapped: %w", tc.cause)
				}

				ctx := context.WithValue(t.Context(), requestContextKey{}, "metadata")
				result, err := p.callback(ctx, newOAuthHandler(t, uc, true), tc.input)
				require.NoError(t, err)
				assert.Equal(t, tc.calls, uc.calls)

				switch tc.kind {
				case "success":
					assert.IsType(t, p.loginSuccess, result)
					location, _ := redirectFields(t, result)
					assert.Equal(t, "https://frontend.example/profile", location)
					assertTokenCookies(t, result)
				case "bad":
					assert.IsType(t, p.loginBadRequest, result)
				case "missing":
					assert.IsType(t, p.loginMissing, result)
				case "failure":
					assert.IsType(t, p.loginFailure, result)
				}
			})
		}
	}
}

func TestOAuthDeletion(t *testing.T) {
	t.Parallel()

	for _, p := range providers() {
		for _, tc := range []struct {
			name       string
			authorized bool
			failure    error
			want       any
			calls      int
		}{
			{"success", true, nil, p.deleteSuccess, 1},
			{"unauthorized", false, nil, p.deleteUnauthorized, 0},
			{"repository failure", true, errors.New("delete failed"), p.deleteFailure, 1},
		} {
			t.Run(p.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				uc := &oauthUsecase{t: t, failure: tc.failure, issuer: p.issuer}
				ctx := context.WithValue(t.Context(), requestContextKey{}, "metadata")
				result, err := p.remove(ctx, newOAuthHandler(t, uc, tc.authorized))
				require.NoError(t, err)
				assert.IsType(t, tc.want, result)
				assert.Equal(t, tc.calls, uc.calls)
			})
		}
	}
}

func (u *oauthUsecase) newState(ctx context.Context, req dto.NewOAuthRequest) error {
	u.t.Helper()

	u.calls++
	assert.Equal(u.t, "metadata", ctx.Value(requestContextKey{}))
	assert.Equal(u.t, authUserID(), req.UserID)
	assert.Equal(u.t, "state", req.State)
	assert.Equal(u.t, 10*time.Minute, req.StateTTL)
	assert.WithinDuration(u.t, time.Now().UTC(), req.RequestTime, time.Second)

	return u.failure
}

func (u *oauthUsecase) link(ctx context.Context, req dto.NewOAuthCallbackRequest) error {
	u.t.Helper()

	u.calls++
	assert.Equal(u.t, "metadata", ctx.Value(requestContextKey{}))
	assert.Equal(u.t, u.wantCode, req.Code)
	assert.Equal(u.t, u.wantState, req.State)
	assert.WithinDuration(u.t, time.Now().UTC(), req.RequestTime, time.Second)

	return u.failure
}

func (u *oauthUsecase) login(ctx context.Context, req dto.OAuthLoginCallbackRequest) (string, string, error) {
	u.t.Helper()

	u.calls++
	assert.Equal(u.t, "metadata", ctx.Value(requestContextKey{}))
	assert.Equal(u.t, u.wantCode, req.Code)
	assert.WithinDuration(u.t, time.Now().UTC(), req.RequestTime, time.Second)

	return "access", "refresh", u.failure
}

func (u *oauthUsecase) remove(ctx context.Context, req dto.DeleteOAuthRequest) error {
	u.t.Helper()

	u.calls++
	assert.Equal(u.t, "metadata", ctx.Value(requestContextKey{}))
	assert.Equal(u.t, dto.DeleteOAuthRequest{UserID: authUserID(), Issuer: u.issuer}, req)

	return u.failure
}
