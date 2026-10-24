package auth_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/usecase/auth"
	"github.com/VasySS/segoya-backend/internal/usecase/auth/mocks"
)

type authHarness struct {
	uc       *auth.Usecase
	users    *mocks.UserRepository
	sessions *mocks.SessionRepository
	tokens   *mocks.TokenService
	crypto   *mocks.CryptoService
	config   auth.Config
	now      time.Time
	profile  user.PrivateProfile
	session  uuid.UUID
}

func newAuthHarness(t *testing.T) *authHarness {
	t.Helper()
	h := &authHarness{
		users: mocks.NewUserRepository(t), sessions: mocks.NewSessionRepository(t),
		tokens: mocks.NewTokenService(t), crypto: mocks.NewCryptoService(t),
		now:     time.Date(2026, time.October, 11, 12, 0, 0, 0, time.UTC),
		session: uuid.MustParse("019a553a-fc00-7000-8000-000000000002"),
		profile: user.PrivateProfile{
			PublicProfile: user.PublicProfile{
				ID:       uuid.MustParse("019a553a-fc00-7000-8000-000000000001"),
				Username: "alice",
				Name:     "Alice",
			},
			Password: "password-hash",
		},
		config: auth.Config{
			RefreshTokenTTL: 24 * time.Hour,
			DiscordNew:      oauth2.Config{ClientID: "discord-link", RedirectURL: "https://app.example/discord/link"},
			DiscordLogin:    oauth2.Config{ClientID: "discord-login", RedirectURL: "https://app.example/discord/login"},
			YandexNew:       oauth2.Config{ClientID: "yandex-link", RedirectURL: "https://app.example/yandex/link"},
			YandexLogin:     oauth2.Config{ClientID: "yandex-login", RedirectURL: "https://app.example/yandex/login"},
		},
	}
	h.uc = auth.NewUsecase(h.config, h.crypto, h.tokens, h.users, h.sessions)

	return h
}

func (h *authHarness) accessClaims() user.AccessTokenClaims {
	return user.AccessTokenClaims{
		SessionID: h.session,
		UserID:    h.profile.ID,
		Username:  h.profile.Username,
		Name:      h.profile.Name,
	}
}

func (h *authHarness) refreshClaims() user.RefreshTokenClaims {
	return user.RefreshTokenClaims{SessionID: h.session, UserID: h.profile.ID, Username: h.profile.Username}
}

func TestUsecase_Login(t *testing.T) {
	t.Parallel()

	for _, failure := range []string{"", "lookup", "password", "access token", "refresh token", "session"} {
		name := failure + " failure"
		if failure == "" {
			name = "success"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newAuthHarness(t)
			cause := errors.New("dependency unavailable")
			req := dto.LoginRequest{RequestTime: h.now, Username: h.profile.Username, Password: "password", UserAgent: "browser"}
			setup := func() {
				if failure == "lookup" {
					h.users.On("GetUserByUsername", mock.Anything, req.Username).Return(user.PrivateProfile{}, cause).Once()
					return
				}

				h.users.On("GetUserByUsername", mock.Anything, req.Username).Return(h.profile, nil).Once()

				if failure == "password" {
					h.crypto.On("CompareHashAndPassword", h.profile.Password, req.Password).Return(cause).Once()
					return
				}

				h.crypto.On("CompareHashAndPassword", h.profile.Password, req.Password).Return(nil).Once()
				h.crypto.On("NewUUID7").Return(h.session).Once()

				if failure == "access token" {
					h.tokens.On("NewAccessToken", h.now, h.accessClaims()).Return("", cause).Once()
					return
				}

				h.tokens.On("NewAccessToken", h.now, h.accessClaims()).Return("access", nil).Once()

				if failure == "refresh token" {
					h.tokens.On("NewRefreshToken", h.now, h.refreshClaims()).Return("", cause).Once()
					return
				}

				h.tokens.On("NewRefreshToken", h.now, h.refreshClaims()).Return("refresh", nil).Once()

				var sessionErr error
				if failure == "session" {
					sessionErr = cause
				}

				h.sessions.On("NewSession", mock.Anything, dto.NewSessionRequest{
					RequestTime: h.now, UserID: h.profile.ID, SessionID: h.session,
					RefreshToken: "refresh", UA: req.UserAgent, Expiration: h.config.RefreshTokenTTL,
				}).Return(sessionErr).Once()
			}
			setup()

			access, refresh, err := h.uc.Login(t.Context(), req)

			if failure != "" {
				want := cause
				if failure == "password" {
					want = user.ErrWrongPassword
				}

				require.ErrorIs(t, err, want)
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

func TestUsecase_Register(t *testing.T) {
	t.Parallel()

	for _, scenario := range []string{
		"success",
		"lookup failure",
		"hash failure",
		"insert failure",
		"existing username",
		"username claimed after lookup",
	} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			h := newAuthHarness(t)
			cause := errors.New("dependency unavailable")
			req := dto.RegisterRequest{RequestTime: h.now, Username: "alice", Name: "Alice", Password: "password"}

			var want error

			switch scenario {
			case "lookup failure":
				h.users.On("GetUserByUsername", mock.Anything, req.Username).Return(user.PrivateProfile{}, cause).Once()
				want = cause
			default:
				// Existing users are returned with nil error; NewUser enforces uniqueness.
				if scenario == "existing username" {
					h.users.On("GetUserByUsername", mock.Anything, req.Username).Return(h.profile, nil).Once()
				} else {
					h.users.On("GetUserByUsername", mock.Anything, req.Username).
						Return(user.PrivateProfile{}, user.ErrUserNotFound).Once()
				}

				if scenario == "hash failure" {
					h.crypto.On("GenerateHashFromPassword", req.Password).Return("", cause).Once()
					want = cause

					break
				}

				h.crypto.On("GenerateHashFromPassword", req.Password).Return("hash", nil).Once()

				if scenario == "insert failure" {
					want = cause
				}

				if scenario == "existing username" || scenario == "username claimed after lookup" {
					want = user.ErrAlreadyExists
				}

				h.users.On("NewUser", mock.Anything, dto.RegisterRequestDB{
					RequestTime: h.now, Username: req.Username, Name: req.Name, Password: "hash",
				}).Return(want).Once()
			}

			require.ErrorIs(t, h.uc.Register(t.Context(), req), want)
		})
	}
}

func TestUsecase_RefreshTokens(t *testing.T) {
	t.Parallel()

	for _, failure := range []string{
		"",
		"parse",
		"session lookup",
		"user lookup",
		"access token",
		"refresh token",
		"session update",
	} {
		name := failure + " failure"
		if failure == "" {
			name = "success"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newAuthHarness(t)
			cause := errors.New("dependency unavailable")
			setup := func() {
				if failure == "parse" {
					h.tokens.On("ParseRefreshToken", "old-refresh").Return(user.RefreshTokenClaims{}, cause).Once()
					return
				}

				h.tokens.On("ParseRefreshToken", "old-refresh").Return(h.refreshClaims(), nil).Once()

				if failure == "session lookup" {
					h.sessions.On("GetSession", mock.Anything, h.profile.ID, h.session).Return(user.Session{}, cause).Once()
					return
				}

				h.sessions.On("GetSession", mock.Anything, h.profile.ID, h.session).
					Return(user.Session{UserID: h.profile.ID, SessionID: h.session, RefreshToken: "old-refresh"}, nil).Once()

				if failure == "user lookup" {
					h.users.On("GetUserByUsername", mock.Anything, h.profile.Username).Return(user.PrivateProfile{}, cause).Once()
					return
				}

				h.users.On("GetUserByUsername", mock.Anything, h.profile.Username).Return(h.profile, nil).Once()

				if failure == "access token" {
					h.tokens.On("NewAccessToken", h.now, h.accessClaims()).Return("", cause).Once()
					return
				}

				h.tokens.On("NewAccessToken", h.now, h.accessClaims()).Return("new-access", nil).Once()

				if failure == "refresh token" {
					h.tokens.On("NewRefreshToken", h.now, h.refreshClaims()).Return("", cause).Once()
					return
				}

				h.tokens.On("NewRefreshToken", h.now, h.refreshClaims()).Return("new-refresh", nil).Once()

				var updateErr error
				if failure == "session update" {
					updateErr = cause
				}

				h.sessions.On("UpdateSession", mock.Anything, dto.UpdateSessionRequest{
					RequestTime: h.now, UserID: h.profile.ID, SessionID: h.session,
					RefreshToken: "new-refresh", Expiration: h.config.RefreshTokenTTL,
				}).Return(updateErr).Once()
			}
			setup()

			access, refresh, err := h.uc.RefreshTokens(t.Context(), dto.TokensRefreshRequest{
				RequestTime:  h.now,
				RefreshToken: "old-refresh",
			})
			if failure != "" {
				require.ErrorIs(t, err, cause)
				assert.Empty(t, access)
				assert.Empty(t, refresh)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, "new-access", access)
			assert.Equal(t, "new-refresh", refresh)
		})
	}
}

func TestUsecase_GetOAuth(t *testing.T) {
	t.Parallel()

	cause := errors.New("oauth lookup unavailable")
	for _, tt := range []struct {
		name string
		want []user.OAuth
		err  error
	}{
		{
			name: "providers returned",
			want: []user.OAuth{
				{OAuthID: "discord-id", Issuer: user.DiscordOAuth},
				{OAuthID: "yandex-id", Issuer: user.YandexOAuth},
			},
		},
		{name: "empty providers", want: []user.OAuth{}},
		{name: "nil providers"},
		{name: "lookup failure", err: cause},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newAuthHarness(t)
			h.users.On("GetOAuth", mock.Anything, h.profile.ID).Return(tt.want, tt.err).Once()
			got, err := h.uc.GetOAuth(t.Context(), h.profile.ID)
			require.ErrorIs(t, err, tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUsecase_GetSessions(t *testing.T) {
	t.Parallel()

	cause := errors.New("sessions unavailable")

	for _, scenario := range []string{"sessions returned", "empty sessions", "nil sessions", "lookup failure"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			h := newAuthHarness(t)

			var (
				want    []user.Session
				wantErr error
			)

			switch scenario {
			case "sessions returned":
				want = []user.Session{{
					UserID: h.profile.ID, SessionID: h.session, UA: "browser", RefreshToken: "refresh", LastActive: h.now,
				}}
			case "empty sessions":
				want = []user.Session{}
			case "lookup failure":
				wantErr = cause
			}

			h.sessions.On("GetSessions", mock.Anything, h.profile.ID).Return(want, wantErr).Once()
			got, err := h.uc.GetSessions(t.Context(), h.profile.ID)
			require.ErrorIs(t, err, wantErr)
			assert.Equal(t, want, got)
		})
	}
}

func TestUsecase_DeleteSession(t *testing.T) {
	t.Parallel()

	for _, scenario := range []string{"success", "lookup failure", "different owner", "delete failure"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			h := newAuthHarness(t)
			cause := errors.New("session repository unavailable")
			session := user.Session{UserID: h.profile.ID, SessionID: h.session}

			var want error

			if scenario == "different owner" {
				session.UserID = uuid.Nil
				want = user.ErrSessionWrongUser
			}

			var lookupErr error
			if scenario == "lookup failure" {
				lookupErr = cause
				want = cause
			}

			lookup := h.sessions.On("GetSession", mock.Anything, h.profile.ID, h.session).Return(session, lookupErr).Once()

			if scenario == "success" || scenario == "delete failure" {
				if scenario == "delete failure" {
					want = cause
				}

				deletion := h.sessions.On("DeleteSession", mock.Anything, h.profile.ID, h.session).Return(want).Once()
				mock.InOrder(lookup, deletion)
			}

			require.ErrorIs(t, h.uc.DeleteSession(t.Context(), h.profile.ID, h.session), want)

			if scenario == "lookup failure" || scenario == "different owner" {
				h.sessions.AssertNotCalled(t, "DeleteSession", mock.Anything, mock.Anything, mock.Anything)
			}
		})
	}
}
