package auth_test

import (
	"errors"
	"testing"
	"time"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/usecase/auth"
	"github.com/VasySS/segoya-backend/internal/usecase/auth/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUsecase_Login(t *testing.T) {
	t.Parallel()

	loginReq := dto.LoginRequest{
		RequestTime: time.Now().UTC(),
		Username:    "username",
		Password:    "password",
		UserAgent:   "userAgent",
	}

	type fields struct {
		conf          auth.Config
		cryptoService *mocks.CryptoService
		tokenService  *mocks.TokenService
		userRepo      *mocks.UserRepository
		sessionRepo   *mocks.SessionRepository
	}

	type args struct {
		req dto.LoginRequest
	}

	tests := []struct {
		name             string
		args             args
		setup            func(fields, args)
		wantAccessToken  string
		wantRefreshToken string
		wantErr          assert.ErrorAssertionFunc
	}{
		{
			name: "successful login",
			args: args{
				req: loginReq,
			},
			setup: func(fs fields, args args) {
				userDB := user.PrivateProfile{
					PublicProfile: user.PublicProfile{
						ID:       1,
						Username: "username",
						Name:     "name",
					},
					Password: "some_password_hash",
				}

				fs.userRepo.On("GetUserByUsername", mock.Anything, args.req.Username).
					Return(userDB, nil)

				fs.cryptoService.On("CompareHashAndPassword", userDB.Password, args.req.Password).
					Return(nil)

				generatedSessionID := "random-session-uuid"

				fs.cryptoService.On("NewUUID4").Return(generatedSessionID)

				fs.tokenService.On("NewAccessToken", mock.Anything, user.AccessTokenClaims{
					SessionID: generatedSessionID,
					UserID:    userDB.ID,
					Username:  userDB.Username,
					Name:      userDB.Name,
				}).Return("accessToken", nil)

				fs.tokenService.On("NewRefreshToken", mock.Anything, user.RefreshTokenClaims{
					SessionID: generatedSessionID,
					UserID:    userDB.ID,
					Username:  userDB.Username,
				}).Return("refreshToken", nil)

				fs.sessionRepo.On("NewSession", mock.Anything, dto.NewSessionRequest{
					RequestTime:  args.req.RequestTime,
					UserID:       userDB.ID,
					SessionID:    generatedSessionID,
					RefreshToken: "refreshToken",
					UA:           args.req.UserAgent,
					Expiration:   fs.conf.RefreshTokenTTL,
				}).Return(nil)
			},
			wantAccessToken:  "accessToken",
			wantRefreshToken: "refreshToken",
			wantErr:          assert.NoError,
		},
		{
			name: "login with wrong password",
			args: args{
				req: loginReq,
			},
			setup: func(fs fields, args args) {
				userDB := user.PrivateProfile{
					PublicProfile: user.PublicProfile{
						ID:       1,
						Username: "username",
						Name:     "name",
					},
					Password: "some_pass_hash",
				}

				fs.userRepo.On("GetUserByUsername", mock.Anything, args.req.Username).
					Return(userDB, nil)

				fs.cryptoService.On("CompareHashAndPassword", userDB.Password, args.req.Password).
					Return(user.ErrWrongPassword)
			},
			wantAccessToken:  "",
			wantRefreshToken: "",
			wantErr: func(tt assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorIs(tt, err, user.ErrWrongPassword)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			userRepo := mocks.NewUserRepository(t)
			sessionRepo := mocks.NewSessionRepository(t)
			tokenService := mocks.NewTokenService(t)
			crypt := mocks.NewCryptoService(t)
			conf := auth.Config{RefreshTokenTTL: time.Hour * 24}
			fs := fields{
				conf:          conf,
				cryptoService: crypt,
				tokenService:  tokenService,
				userRepo:      userRepo,
				sessionRepo:   sessionRepo,
			}
			tt.setup(fs, tt.args)

			uc := auth.NewUsecase(fs.conf, fs.cryptoService, fs.tokenService, fs.userRepo, fs.sessionRepo)

			accessToken, refreshToken, err := uc.Login(t.Context(), tt.args.req)
			tt.wantErr(t, err)
			assert.Equal(t, tt.wantAccessToken, accessToken)
			assert.Equal(t, tt.wantRefreshToken, refreshToken)
		})
	}
}

func TestUsecase_Register(t *testing.T) {
	t.Parallel()

	registerReq := dto.RegisterRequest{
		RequestTime: time.Now().UTC(),
		Username:    "username",
		Name:        "name",
		Password:    "password",
	}

	type fields struct {
		conf         auth.Config
		cryptService *mocks.CryptoService
		tokenService *mocks.TokenService
		userRepo     *mocks.UserRepository
		sessionRepo  *mocks.SessionRepository
	}

	type args struct {
		req dto.RegisterRequest
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully register user",
			args: args{
				req: registerReq,
			},
			setup: func(fs fields, args args) {
				fs.userRepo.On("GetUserByUsername", mock.Anything, args.req.Username).
					Return(user.PrivateProfile{
						Password: "some_hash_from_db",
					}, user.ErrUserNotFound)

				fs.cryptService.On("GenerateHashFromPassword", args.req.Password).
					Return("some_hash", nil)

				fs.userRepo.On("NewUser", mock.Anything, dto.RegisterRequestDB{
					RequestTime: args.req.RequestTime,
					Username:    args.req.Username,
					Name:        args.req.Name,
					Password:    "some_hash",
				}).Return(nil)
			},
			wantErr: assert.NoError,
		},
		{
			name: "trying to register user with existing username",
			args: args{
				req: registerReq,
			},
			setup: func(fs fields, args args) {
				fs.userRepo.On("GetUserByUsername", mock.Anything, args.req.Username).
					Return(user.PrivateProfile{}, user.ErrAlreadyExists)
			},
			wantErr: func(tt assert.TestingT, err error, _ ...any) bool {
				return assert.ErrorIs(tt, err, user.ErrAlreadyExists)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			userRepo := mocks.NewUserRepository(t)
			rnd := mocks.NewCryptoService(t)
			fs := fields{
				cryptService: rnd,
				userRepo:     userRepo,
			}
			tt.setup(fs, tt.args)

			uc := auth.NewUsecase(fs.conf, fs.cryptService, fs.tokenService, fs.userRepo, fs.sessionRepo)

			err := uc.Register(t.Context(), tt.args.req)
			tt.wantErr(t, err)
		})
	}
}

func TestUsecase_RefreshTokens(t *testing.T) {
	t.Parallel()

	type fields struct {
		conf         auth.Config
		tokenService *mocks.TokenService
		userRepo     *mocks.UserRepository
		sessionRepo  *mocks.SessionRepository
	}

	type args struct {
		req dto.TokensRefreshRequest
	}

	tests := []struct {
		name             string
		args             args
		setup            func(fields, args)
		wantAccessToken  string
		wantRefreshToken string
		wantErr          assert.ErrorAssertionFunc
	}{
		{
			name: "successfully refresh tokens",
			args: args{
				req: dto.TokensRefreshRequest{
					RequestTime:  time.Now().UTC(),
					RefreshToken: "refresh_token",
				},
			},
			setup: func(fs fields, args args) {
				refreshTokenClaims := user.RefreshTokenClaims{
					SessionID: "session_id",
					UserID:    1,
					Username:  "username",
				}

				fs.tokenService.On("ParseRefreshToken", args.req.RefreshToken).
					Return(refreshTokenClaims, nil)

				existingSession := user.Session{
					UserID:       refreshTokenClaims.UserID,
					RefreshToken: "refresh_token",
					SessionID:    refreshTokenClaims.SessionID,
					UA:           "some_user_agent",
				}

				fs.sessionRepo.On("GetSession", mock.Anything,
					refreshTokenClaims.UserID, refreshTokenClaims.SessionID).
					Return(existingSession, nil)

				fs.userRepo.On("GetUserByUsername", mock.Anything, refreshTokenClaims.Username).
					Return(user.PrivateProfile{
						PublicProfile: user.PublicProfile{
							ID:       refreshTokenClaims.UserID,
							Name:     "name",
							Username: "username",
						},
					}, nil)

				fs.tokenService.On("NewAccessToken", args.req.RequestTime, user.AccessTokenClaims{
					SessionID: refreshTokenClaims.SessionID,
					UserID:    refreshTokenClaims.UserID,
					Username:  "username",
					Name:      "name",
				}).Return("access_token", nil)

				fs.tokenService.On("NewRefreshToken", args.req.RequestTime, user.RefreshTokenClaims{
					SessionID: refreshTokenClaims.SessionID,
					UserID:    refreshTokenClaims.UserID,
					Username:  "username",
				}).Return("refresh_token", nil)

				fs.sessionRepo.On("UpdateSession", mock.Anything, dto.UpdateSessionRequest{
					RequestTime:  args.req.RequestTime,
					UserID:       refreshTokenClaims.UserID,
					RefreshToken: "refresh_token",
					SessionID:    refreshTokenClaims.SessionID,
					Expiration:   fs.conf.RefreshTokenTTL,
				}).Return(nil)
			},
			wantAccessToken:  "access_token",
			wantRefreshToken: "refresh_token",
			wantErr:          assert.NoError,
		},
		{
			name: "trying to refresh tokens with expired session",
			args: args{
				req: dto.TokensRefreshRequest{
					RequestTime:  time.Now().UTC(),
					RefreshToken: "refresh_token",
				},
			},
			setup: func(fs fields, args args) {
				fs.tokenService.On("ParseRefreshToken", args.req.RefreshToken).
					Return(user.RefreshTokenClaims{
						UserID:    1,
						SessionID: "session_id",
					}, nil)

				fs.sessionRepo.On("GetSession", mock.Anything, 1, "session_id").
					Return(user.Session{}, errors.New("session not found"))
			},
			wantAccessToken:  "",
			wantRefreshToken: "",
			wantErr:          assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			userRepo := mocks.NewUserRepository(t)
			tokenService := mocks.NewTokenService(t)
			sessionRepo := mocks.NewSessionRepository(t)
			conf := auth.Config{
				RefreshTokenTTL: time.Hour,
			}
			fs := fields{
				conf:         conf,
				sessionRepo:  sessionRepo,
				tokenService: tokenService,
				userRepo:     userRepo,
			}
			tt.setup(fs, tt.args)

			uc := auth.NewUsecase(fs.conf, nil, fs.tokenService, fs.userRepo, fs.sessionRepo)

			accessToken, refreshToken, err := uc.RefreshTokens(t.Context(), tt.args.req)
			tt.wantErr(t, err)
			assert.Equal(t, tt.wantAccessToken, accessToken)
			assert.Equal(t, tt.wantRefreshToken, refreshToken)
		})
	}
}

func TestUsecase_GetOAuth(t *testing.T) {
	t.Parallel()

	oauthInfoResponse := []user.OAuth{
		{OAuthID: "oauth_id1", Issuer: "provider1"},
		{OAuthID: "oauth_id2", Issuer: "provider2"},
	}

	type fields struct {
		userRepo *mocks.UserRepository
	}

	type args struct {
		userID int
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    []user.OAuth
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully get oauth info",
			args: args{
				userID: 1,
			},
			setup: func(fs fields, args args) {
				fs.userRepo.On("GetOAuth", mock.Anything, args.userID).
					Return(oauthInfoResponse, nil)
			},
			want:    oauthInfoResponse,
			wantErr: assert.NoError,
		},
		{
			name: "failed to get oauth info",
			args: args{
				userID: 1,
			},
			setup: func(fs fields, args args) {
				fs.userRepo.On("GetOAuth", mock.Anything, args.userID).
					Return(nil, errors.New("failed to get oauth info"))
			},
			want:    nil,
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			userRepo := mocks.NewUserRepository(t)
			fs := fields{
				userRepo: userRepo,
			}
			tt.setup(fs, tt.args)

			uc := auth.NewUsecase(auth.Config{}, nil, nil, fs.userRepo, nil)

			_, err := uc.GetOAuth(t.Context(), tt.args.userID)
			tt.wantErr(t, err)
		})
	}
}

func TestUsecase_GetSessions(t *testing.T) {
	t.Parallel()

	userSessionsResponse := []user.Session{
		{UserID: 1, SessionID: "session_id1"},
		{UserID: 2, SessionID: "session_id2"},
	}

	type fields struct {
		sessionRepo *mocks.SessionRepository
	}

	type args struct {
		userID int
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		want    []user.Session
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully get user session",
			args: args{
				userID: 1,
			},
			setup: func(fs fields, args args) {
				fs.sessionRepo.On("GetSessions", mock.Anything, args.userID, mock.Anything).
					Return(userSessionsResponse, nil)
			},
			want:    userSessionsResponse,
			wantErr: assert.NoError,
		},
		{
			name: "failed to get user session",
			args: args{
				userID: 1,
			},
			setup: func(fs fields, args args) {
				fs.sessionRepo.On("GetSessions", mock.Anything, args.userID, mock.Anything).
					Return([]user.Session(nil), errors.New("failed to get user session"))
			},
			want:    []user.Session(nil),
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sessionRepo := mocks.NewSessionRepository(t)
			fs := fields{
				sessionRepo: sessionRepo,
			}
			tt.setup(fs, tt.args)

			uc := auth.NewUsecase(auth.Config{}, nil, nil, nil, fs.sessionRepo)

			_, err := uc.GetSessions(t.Context(), tt.args.userID)
			tt.wantErr(t, err)
		})
	}
}

func TestUsecase_DeleteSession(t *testing.T) {
	t.Parallel()

	type fields struct {
		sessionRepo *mocks.SessionRepository
	}

	type args struct {
		userID    int
		sessionID string
	}

	tests := []struct {
		name    string
		args    args
		setup   func(fields, args)
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successfully delete user session",
			args: args{
				userID:    1,
				sessionID: "session_id",
			},
			setup: func(fs fields, args args) {
				fs.sessionRepo.On("GetSession", mock.Anything, args.userID, args.sessionID).
					Return(user.Session{UserID: args.userID, SessionID: args.sessionID}, nil)

				fs.sessionRepo.On("DeleteSession", mock.Anything, args.userID, args.sessionID).
					Return(nil)
			},
			wantErr: assert.NoError,
		},
		{
			name: "trying to get user session of another user",
			args: args{
				userID:    1,
				sessionID: "session_id",
			},
			setup: func(fs fields, args args) {
				fs.sessionRepo.On("GetSession", mock.Anything, args.userID, args.sessionID).
					Return(user.Session{UserID: 2, SessionID: args.sessionID}, nil)
			},
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sessionRepo := mocks.NewSessionRepository(t)
			fs := fields{
				sessionRepo: sessionRepo,
			}
			tt.setup(fs, tt.args)

			uc := auth.NewUsecase(auth.Config{}, nil, nil, nil, fs.sessionRepo)

			err := uc.DeleteSession(t.Context(), tt.args.userID, tt.args.sessionID)
			tt.wantErr(t, err)
		})
	}
}
