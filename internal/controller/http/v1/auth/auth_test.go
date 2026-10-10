package auth_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/config"
	httpController "github.com/VasySS/segoya-backend/internal/controller/http"
	"github.com/VasySS/segoya-backend/internal/controller/http/v1/auth"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport/melody"
	"github.com/VasySS/segoya-backend/pkg/captcha"
	httpPkg "github.com/VasySS/segoya-backend/pkg/http"
)

type authUsecase struct {
	auth.Usecase

	login    func(context.Context, dto.LoginRequest) (string, string, error)
	register func(context.Context, dto.RegisterRequest) error
}

func (u authUsecase) Login(ctx context.Context, req dto.LoginRequest) (string, string, error) {
	return u.login(ctx, req)
}

func (u authUsecase) Register(ctx context.Context, req dto.RegisterRequest) error {
	return u.register(ctx, req)
}

type authRoundTripFunc func(*http.Request) (*http.Response, error)

func (f authRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type requestContextKey struct{}

func TestAuthentication_CaptchaSubmissions(t *testing.T) {
	t.Parallel()

	providers := []struct {
		name       string
		field      string
		success    string
		failure    string
		newService func(*http.Client, string) auth.CaptchaService
	}{
		{
			name: "turnstile", field: "response", success: `{"success":true}`,
			failure: `{"success":false,"error-codes":["timeout-or-duplicate"]}`,
			newService: func(client *http.Client, secret string) auth.CaptchaService {
				return captcha.NewCloudflareService(client, secret)
			},
		},
		{
			name: "yandex", field: "token", success: `{"status":"ok"}`,
			failure: `{"status":"failed","message":"Invalid or expired Token."}`,
			newService: func(client *http.Client, secret string) auth.CaptchaService {
				return captcha.NewYandexService(client, secret)
			},
		},
	}

	for _, provider := range providers {
		for _, operation := range []string{"login", "register"} {
			t.Run(provider.name+"/"+operation, func(t *testing.T) {
				t.Parallel()

				var verificationCalls, businessCalls int

				tokens := []string{}
				client := httpPkg.NewClient()
				client.Transport = authRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					verificationCalls++

					require.NoError(t, req.ParseForm())
					assert.Equal(t, "secret", req.PostForm.Get("secret"))
					assert.Equal(t, "request metadata", req.Context().Value(requestContextKey{}))
					tokens = append(tokens, req.PostForm.Get(provider.field))

					body := provider.failure
					if verificationCalls == 2 || verificationCalls == 4 {
						body = provider.success
					}

					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
				})
				usecase := authUsecase{
					login: func(ctx context.Context, req dto.LoginRequest) (string, string, error) {
						businessCalls++
						assert.Equal(t, businessCalls*2, verificationCalls, "captcha must run before business logic")
						assert.Equal(t, "request metadata", ctx.Value(requestContextKey{}))
						assert.Equal(t, "username", req.Username)
						assert.Equal(t, "password", req.Password)
						assert.Equal(t, "test-agent", req.UserAgent)
						assert.False(t, req.RequestTime.IsZero())

						return "access-token", "refresh-token", nil
					},
					register: func(ctx context.Context, req dto.RegisterRequest) error {
						businessCalls++
						assert.Equal(t, businessCalls*2, verificationCalls, "captcha must run before business logic")
						assert.Equal(t, "request metadata", ctx.Value(requestContextKey{}))
						assert.Equal(t, "username", req.Username)
						assert.Equal(t, "password", req.Password)
						assert.Equal(t, "display name", req.Name)
						assert.False(t, req.RequestTime.IsZero())

						return nil
					},
				}

				var conf config.Config

				conf.ENV.FrontendURL = url.URL{Scheme: "http", Host: "localhost:5173"}
				conf.Limits.AccessTokenTTL = time.Minute
				conf.Limits.RefreshTokenTTL = time.Hour
				lobbyWS := melody.NewWebSocketService()
				multiplayerWS := melody.NewWebSocketService()

				t.Cleanup(func() { require.NoError(t, lobbyWS.Close()) })
				t.Cleanup(func() { require.NoError(t, multiplayerWS.Close()) })

				router := httpController.NewRouter(
					conf,
					nil,
					nil,
					provider.newService(client, "secret"),
					lobbyWS,
					multiplayerWS,
					usecase,
					nil,
					nil,
					nil,
					nil,
				)

				ctx := context.WithValue(t.Context(), requestContextKey{}, "request metadata")
				for i, token := range []string{"rejected-token", "accepted-token", "accepted-token", "fresh-token"} {
					req := httptest.NewRequestWithContext(ctx,
						http.MethodPost,
						"/v1/auth/"+operation,
						strings.NewReader(`{"username":"username","password":"password","name":"display name"}`),
					)
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("User-Agent", "test-agent")
					req.Header.Set("Frontend-Captcha-Token", token)

					recorder := httptest.NewRecorder()
					router.ServeHTTP(recorder, req)

					if i%2 == 0 {
						assert.Equal(t, http.StatusBadRequest, recorder.Code)
						assert.JSONEq(t, `{"title":"Captcha validation failed","status":400,`+
							`"detail":"Captcha validation failed, please try again"}`, recorder.Body.String())

						continue
					}

					wantStatus := http.StatusNoContent
					if operation == "register" {
						wantStatus = http.StatusCreated
					}

					assert.Equal(t, wantStatus, recorder.Code)
					assert.Empty(t, recorder.Body.String())

					if operation == "login" {
						assert.Contains(t, recorder.Header().Get("Set-Cookie"), "accessToken=access-token")
						assert.Contains(t, recorder.Header().Get("Set-Cookie"), "refreshToken=refresh-token")
					}
				}

				assert.Equal(t, []string{"rejected-token", "accepted-token", "accepted-token", "fresh-token"}, tokens)
				assert.Equal(t, 4, verificationCalls)
				assert.Equal(t, 2, businessCalls)
			})
		}
	}
}

func TestAuthentication_BusinessRejectionConsumesToken(t *testing.T) {
	t.Parallel()

	for _, operation := range []string{"login", "register"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()

			var verificationCalls, businessCalls int

			client := httpPkg.NewClient()
			client.Transport = authRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
				verificationCalls++

				body := `{"success":true}`
				if verificationCalls == 2 {
					body = `{"success":false}`
				}

				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			usecase := authUsecase{
				login: func(_ context.Context, _ dto.LoginRequest) (string, string, error) {
					businessCalls++
					return "", "", user.ErrWrongPassword
				},
				register: func(_ context.Context, _ dto.RegisterRequest) error {
					businessCalls++
					return user.ErrAlreadyExists
				},
			}

			handler := auth.NewHandler(auth.Config{}, usecase, nil, nil, captcha.NewCloudflareService(client, "secret"))
			for i, token := range []string{"same-token", "same-token", "fresh-token"} {
				testBusinessRejection(t, handler, operation, token, i == 1)
			}

			assert.Equal(t, 3, verificationCalls)
			assert.Equal(t, 2, businessCalls)
		})
	}
}

func testBusinessRejection(t *testing.T, handler *auth.Handler, operation, token string, captchaRejected bool) {
	t.Helper()

	if operation == "login" {
		result, err := handler.Login(t.Context(), &api.LoginRequest{}, api.LoginParams{
			FrontendCaptchaToken: api.NewOptString(token),
		})
		require.NoError(t, err)

		if captchaRejected {
			assert.IsType(t, &api.LoginBadRequest{}, result)
			return
		}

		assert.IsType(t, &api.LoginUnauthorized{}, result)

		return
	}

	result, err := handler.Register(t.Context(), &api.RegisterRequest{}, api.RegisterParams{
		FrontendCaptchaToken: api.NewOptString(token),
	})
	require.NoError(t, err)

	if captchaRejected {
		assert.IsType(t, &api.RegisterBadRequest{}, result)
		return
	}

	assert.IsType(t, &api.RegisterConflict{}, result)
}
