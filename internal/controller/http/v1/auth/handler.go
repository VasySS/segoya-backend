// Package auth contains HTTP handlers for authentication and authorization.
package auth

import (
	"context"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// DiscordOAuth defines methods for handling Discord OAuth authentication and authorization flows.
type DiscordOAuth interface {
	NewDiscord(ctx context.Context, req dto.NewOAuthRequest) error
	NewDiscordCallback(ctx context.Context, req dto.NewOAuthCallbackRequest) error
	LoginDiscordCallback(ctx context.Context, req dto.OAuthLoginCallbackRequest) (access string, refresh string, err error)
	DeleteDiscord(ctx context.Context, req dto.DeleteOAuthRequest) error
}

// YandexOAuth defines methods for handling Yandex OAuth authentication and authorization flows.
type YandexOAuth interface {
	NewYandex(ctx context.Context, req dto.NewOAuthRequest) error
	NewYandexCallback(ctx context.Context, req dto.NewOAuthCallbackRequest) error
	LoginYandexCallback(ctx context.Context, req dto.OAuthLoginCallbackRequest) (access string, refresh string, err error)
	DeleteYandex(ctx context.Context, req dto.DeleteOAuthRequest) error
}

// DefaultAuth defines methods for handling standard username/password-based authentication.
type DefaultAuth interface {
	Register(ctx context.Context, userReq dto.RegisterRequest) error
	Login(ctx context.Context, user dto.LoginRequest) (access string, refresh string, err error)
	RefreshTokens(ctx context.Context, req dto.TokensRefreshRequest) (access string, refresh string, err error)
	GetOAuth(ctx context.Context, userID int) ([]user.OAuth, error)
	GetSessions(ctx context.Context, userID int) ([]user.Session, error)
	DeleteSession(ctx context.Context, userID int, sessionID string) error
}

// Usecase consolidates all authentication use cases into a single interface.
type Usecase interface {
	DiscordOAuth
	YandexOAuth
	DefaultAuth
}

// TokenService defines the interface for handling user JWT token operations.
type TokenService interface {
	FromContext(ctx context.Context) (user.AccessTokenClaims, bool)
}

// RandomGenerator defines methods for generating random strings.
type RandomGenerator interface {
	NewRandomHexString(length int) string
}

// CaptchaService defines methods for verifying captcha tokens.
type CaptchaService interface {
	IsTokenValid(ctx context.Context, token string) error
}

var _ api.AuthHandler = (*Handler)(nil)

// Handler implements the api.AuthHandler interface and handles HTTP requests for user operations.
type Handler struct {
	cfg Config
	uc  Usecase
	ts  TokenService
	rnd RandomGenerator
	cs  CaptchaService
}

// NewHandler creates and returns a new Handler instance with the provided dependencies.
//
// cfg - Configuration settings for the Handler.
//
// uc - Implementation of the Usecase interface for business logic.
//
// rnd - Implementation of the RandomGenerator interface for generating random strings.
//
// tokenService - Implementation of the TokenService interface for handling tokens.
//
// captchaService - Implementation of the CaptchaService interface for handling captcha tokens.
func NewHandler(
	cfg Config,
	uc Usecase,
	rnd RandomGenerator,
	tokenService TokenService,
	captchaService CaptchaService,
) *Handler {
	return &Handler{
		cfg: cfg,
		uc:  uc,
		rnd: rnd,
		ts:  tokenService,
		cs:  captchaService,
	}
}
