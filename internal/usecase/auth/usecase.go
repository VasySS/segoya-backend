// Package auth provides authentication and authorization services including
// user registration, login, session management, token handling, and OAuth integration.
package auth

import (
	"context"
	"time"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// UserRepository defines methods for interacting with user data in the storage layer.
//
//go:generate go tool mockery --name=UserRepository
type UserRepository interface {
	NewUser(ctx context.Context, req dto.RegisterRequestDB) error
	GetUserByUsername(ctx context.Context, username string) (user.PrivateProfile, error)
	NewOAuth(ctx context.Context, req dto.NewOAuthRequestDB) error
	GetOAuth(ctx context.Context, userID int) ([]user.OAuth, error)
	DeleteOAuth(ctx context.Context, req dto.DeleteOAuthRequest) error
	GetUserByOAuth(ctx context.Context, req dto.GetUserByOAuthRequest) (user.PrivateProfile, error)
}

// SessionRepository defines methods for interacting with user sessions in the storage layer.
//
//go:generate go tool mockery --name=SessionRepository
type SessionRepository interface {
	NewSession(ctx context.Context, req dto.NewSessionRequest) error
	GetSession(ctx context.Context, userID int, sessionID string) (user.Session, error)
	GetSessions(ctx context.Context, userID int) ([]user.Session, error)
	UpdateSession(ctx context.Context, req dto.UpdateSessionRequest) error
	DeleteSession(ctx context.Context, userID int, sessionID string) error
	NewOAuthState(ctx context.Context, req dto.NewOAuthRequest) error
	GetOAuthUserID(ctx context.Context, state string) (int, error)
}

// CryptoService defines methods for cryptographic operations such as password hashing and UUID generation.
//
//go:generate go tool mockery --name=CryptoService
type CryptoService interface {
	NewUUID4() string
	CompareHashAndPassword(hash, password string) error
	GenerateHashFromPassword(password string) (string, error)
}

// TokenService defines methods for generating and parsing access and refresh tokens.
//
//go:generate go tool mockery --name=TokenService
type TokenService interface {
	NewAccessToken(current time.Time, req user.AccessTokenClaims) (string, error)
	NewRefreshToken(current time.Time, req user.RefreshTokenClaims) (string, error)
	ParseAccessToken(token string) (user.AccessTokenClaims, error)
	ParseRefreshToken(token string) (user.RefreshTokenClaims, error)
}

// Usecase contains authentication business logic and dependencies.
type Usecase struct {
	conf          Config
	cryptoService CryptoService
	tokenService  TokenService
	userRepo      UserRepository
	sessionRepo   SessionRepository
	tracer        trace.Tracer
}

// NewUsecase creates and returns a new instance of Usecase with the provided dependencies.
//
// conf - Configuration settings for the Usecase.
//
// rnd - Instance of CryptoService for cryptographic operations.
//
// tokenService - Instance of TokenService for handling tokens.
//
// userRepo - Instance of UserRepository for interacting with user data.
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
