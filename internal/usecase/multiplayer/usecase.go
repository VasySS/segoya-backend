// Package multiplayer manages multiplayer game sessions including game lifecycle,
// round management, player guesses.
package multiplayer

import (
	"context"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// GameRepo provides access to multiplayer game data.
type GameRepo interface {
	LockMultiplayerGame(ctx context.Context, gameID int) error
	NewMultiplayerGame(ctx context.Context, req dto.NewMultiplayerGameRequest) (int, error)
	GetMultiplayerGame(ctx context.Context, id int) (multiplayer.Game, error)
	EndMultiplayerGame(ctx context.Context, req dto.EndMultiplayerGameRequestDB) error
	GetMultiplayerGameUser(ctx context.Context, userID, gameID int) (user.MultiplayerUser, error)
	GetMultiplayerGameUsers(ctx context.Context, gameID int) ([]user.MultiplayerUser, error)
	GetMultiplayerGameGuesses(ctx context.Context, gameID int) ([]multiplayer.Guess, error)
}

// RoundRepo provides access to multiplayer round data.
type RoundRepo interface {
	NewMultiplayerRound(ctx context.Context, req dto.NewMultiplayerRoundRequestDB) (multiplayer.Round, error)
	GetMultiplayerRound(ctx context.Context, gameID, roundNum int) (multiplayer.Round, error)
	EndMultiplayerRound(ctx context.Context, req dto.EndMultiplayerRoundRequestDB) error
	NewMultiplayerRoundGuess(ctx context.Context, req dto.NewMultiplayerRoundGuessRequestDB) error
	GetMultiplayerRoundGuesses(ctx context.Context, roundID int) ([]multiplayer.Guess, error)
}

// Repository provides access to both game and round data, and includes transaction management.
//
//go:generate go tool mockery --name=Repository
type Repository interface {
	repository.TxManager
	RoundRepo
	GameRepo
}

// PanoramaUsecase defines methods for interacting with streetview panoramas and calculating scores.
//
//go:generate go tool mockery --name=PanoramaUsecase
type PanoramaUsecase interface {
	NewStreetview(ctx context.Context, provider game.PanoramaProvider) (game.PanoramaMetadata, error)
	GetStreetview(ctx context.Context, provider game.PanoramaProvider, id int) (game.PanoramaMetadata, error)
	CalculateScoreAndDistance(
		provider game.PanoramaProvider,
		realLat, realLng, userLat, userLng float64,
	) (score int, distance int)
}

// Usecase contains business logic for multiplayer game management.
type Usecase struct {
	cfg    Config
	repo   Repository
	pano   PanoramaUsecase
	tracer trace.Tracer
}

// NewUsecase creates and returns a new Usecase instance with the provided dependencies.
//
// cfg - Configuration settings for the multiplayer game management.
// repo - Implementation of the Repository interface for accessing game and round data.
// pano - Implementation of the PanoramaUsecase interface for panorama-based gameplay interactions.
func NewUsecase(cfg Config, repo Repository, pano PanoramaUsecase) *Usecase {
	return &Usecase{
		cfg:    cfg,
		repo:   repo,
		pano:   pano,
		tracer: otel.GetTracerProvider().Tracer("MultiplayerUsecase"),
	}
}
