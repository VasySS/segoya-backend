// Package auth provides authentication and authorization services including
// user registration, login, session management, token handling, and OAuth integration.
package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/oauth2"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// UserRepository defines methods for retrieving user data from the repository layer.
//
//go:generate go tool mockery --name=UserRepository
type UserRepository interface {
	NewUser(ctx context.Context, req dto.RegisterRequestDB) error
	GetUserByUsername(ctx context.Context, username string) (user.PrivateProfile, error)
	NewOAuth(ctx context.Context, req dto.NewOAuthRequestDB) error
	GetOAuth(ctx context.Context, userID uuid.UUID) ([]user.OAuth, error)
	DeleteOAuth(ctx context.Context, req dto.DeleteOAuthRequest) error
	GetUserByOAuth(ctx context.Context, req dto.GetUserByOAuthRequest) (user.PrivateProfile, error)
}

// SessionRepository defines methods for managing user sessions in the repository layer.
//
//go:generate go tool mockery --name=SessionRepository
type SessionRepository interface {
	NewSession(ctx context.Context, req dto.NewSessionRequest) error
	GetSession(ctx context.Context, userID, sessionID uuid.UUID) (user.Session, error)
	GetSessions(ctx context.Context, userID uuid.UUID) ([]user.Session, error)
	UpdateSession(ctx context.Context, req dto.UpdateSessionRequest) error
	DeleteSession(ctx context.Context, userID, sessionID uuid.UUID) error
	NewOAuthState(ctx context.Context, req dto.NewOAuthRequest) error
	GetOAuthUserID(ctx context.Context, state string) (uuid.UUID, error)
}

// CryptoService defines methods for working with cryptographic operations.
//
//go:generate go tool mockery --name=CryptoService
type CryptoService interface {
	NewUUID7() uuid.UUID
	CompareHashAndPassword(hash, password string) error
	GenerateHashFromPassword(password string) (string, error)
}

// TokenService defines methods for working with JWT tokens.
//
//go:generate go tool mockery --name=TokenService
type TokenService interface {
	NewAccessToken(current time.Time, req user.AccessTokenClaims) (string, error)
	NewRefreshToken(current time.Time, req user.RefreshTokenClaims) (string, error)
	ParseAccessToken(token string) (user.AccessTokenClaims, error)
	ParseRefreshToken(token string) (user.RefreshTokenClaims, error)
	ExchangeDiscordCodeForID(ctx context.Context, config oauth2.Config, code string) (string, error)
	ExchangeYandexCodeForID(ctx context.Context, config oauth2.Config, code string) (string, error)
}

// Usecase contains authentication business logic and its dependencies.
type Usecase struct {
	conf          Config
	cryptoService CryptoService
	tokenService  TokenService
	userRepo      UserRepository
	sessionRepo   SessionRepository
	tracer        trace.Tracer
}

// NewUsecase creates and returns a new instance of auth usecase with the provided dependencies.
//
// conf - Configuration settings for the usecase.
//
// rnd - Instance of CryptoService for cryptographic operations.
//
// tokenService - Instance of TokenService for JWT tokens handling.
//
// userRepo - Instance of UserRepository for working with user data.
//
// sessionRepo - Instance of SessionRepository for managing user sessions.
func NewUsecase(
	conf Config,
	rnd CryptoService,
	tokenService TokenService,
	userRepo UserRepository,
	sessionRepo SessionRepository,
) *Usecase {
	return &Usecase{
		conf:          conf,
		cryptoService: rnd,
		tokenService:  tokenService,
		userRepo:      userRepo,
		sessionRepo:   sessionRepo,
		tracer:        otel.GetTracerProvider().Tracer("AuthUsecase"),
	}
}
