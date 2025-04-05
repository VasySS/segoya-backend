// Package lobby provides functionality for managing multiplayer game lobbies,
// including creation, joining, expiration management, and transitioning lobbies
// to active games.
package lobby

import (
	"context"
	"time"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/lobby"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Repository provides access to and management of lobby data including
// player count tracking, expiration timers, and persistence operations.
//
//go:generate go tool mockery --name=Repository
type Repository interface {
	NewLobby(ctx context.Context, req dto.NewLobbyRequestDB) error
	GetLobby(ctx context.Context, id string) (lobby.Lobby, error)
	DeleteLobby(ctx context.Context, id string) error
	GetLobbies(ctx context.Context, req dto.GetLobbiesRequest) ([]lobby.Lobby, int, error)
	IncrementLobbyPlayers(ctx context.Context, id string) error
	DecrementLobbyPlayers(ctx context.Context, id string) error
	AddLobbyExpiration(ctx context.Context, id string, ttl time.Duration) error
	DeleteLobbyExpiration(ctx context.Context, id string) error
}

// UserRepository provides access to user data.
//
//go:generate go tool mockery --name=UserRepository
type UserRepository interface {
	GetUserByID(ctx context.Context, id int) (user.PrivateProfile, error)
}

// MultiplayerUsecase provides method for creating multiplayer games.
//
//go:generate go tool mockery --name=MultiplayerUsecase
type MultiplayerUsecase interface {
	NewGame(ctx context.Context, req dto.NewMultiplayerGameRequest) (int, error)
}

// RandomGenerator provides cryptographically secure random string generation.
//
//go:generate go tool mockery --name=RandomGenerator
type RandomGenerator interface {
	NewRandomHexString(length int) string
}

// Usecase contains business logic for lobby management.
type Usecase struct {
	conf      Config
	rnd       RandomGenerator
	lobbyRepo Repository
	userRepo  UserRepository
	mult      MultiplayerUsecase
	tracer    trace.Tracer
}

// NewUsecase creates and returns a new instance of Usecase with the provided dependencies.
//
// conf - Configuration settings for the Usecase.
//
// rnd - Instance of RandomGenerator for generating random strings.
//
// userRepo - Implementation of UserRepository for accessing user data.
//
// lobbyRepo - Implementation of LobbyRepository for managing lobby data.
//
// mult - Implementation of MultiplayerUsecase for managing multiplayer games.
func NewUsecase(
	conf Config,
	rnd RandomGenerator,
	userRepo UserRepository,
	lobbyRepo Repository,
	mult MultiplayerUsecase,
) *Usecase {
	return &Usecase{
		conf:      conf,
		rnd:       rnd,
		lobbyRepo: lobbyRepo,
		userRepo:  userRepo,
		mult:      mult,
		tracer:    otel.GetTracerProvider().Tracer("LobbyUsecase"),
	}
}
