// Package singleplayer manages singleplayer game sessions including game lifecycle,
// round management, player guesses.
package singleplayer

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository"
	"github.com/google/uuid"
)

// GameRepo defines methods for accessing and modifying singleplayer game data.
type GameRepo interface {
	LockSingleplayerGame(ctx context.Context, gameID uuid.UUID) error
	NewSingleplayerGame(ctx context.Context, req dto.NewSingleplayerGameRequest) (uuid.UUID, error)
	GetSingleplayerGame(ctx context.Context, gameID uuid.UUID) (singleplayer.Game, error)
	GetSingleplayerGames(
		ctx context.Context,
		req dto.GetSingleplayerGamesRequest,
	) ([]singleplayer.Game, int, error)
	EndSingleplayerGame(ctx context.Context, req dto.EndSingleplayerGameRequestDB) error
}

// RoundRepo defines methods for accessing and modifying singleplayer round data.
type RoundRepo interface {
	NewSingleplayerRound(ctx context.Context, req dto.NewSingleplayerRoundDBRequest) (singleplayer.Round, error)
	GetSingleplayerRound(ctx context.Context, gameID uuid.UUID, roundNum int) (singleplayer.Round, error)
	GetSingleplayerGameGuesses(ctx context.Context, gameID uuid.UUID) ([]singleplayer.Guess, error)
	NewSingleplayerRoundGuess(ctx context.Context, req dto.NewSingleplayerRoundGuessRequest) error
}

// Repository provides access to game and round data.
//
//go:generate go tool mockery --name=Repository
type Repository interface {
	repository.TxManager
	GameRepo
	RoundRepo
}

// PanoramaUsecase defines methods for interacting with streetview panoramas and calculating scores.
//
//go:generate go tool mockery --name=PanoramaUsecase
type PanoramaUsecase interface {
	NewStreetview(ctx context.Context, provider game.PanoramaProvider) (game.PanoramaMetadata, error)
	GetStreetview(ctx context.Context, provider game.PanoramaProvider, id uuid.UUID) (game.PanoramaMetadata, error)
	CalculateScoreAndDistance(
		provider game.PanoramaProvider,
		realLat, realLng, userLat, userLng float64,
	) (score int, distance int)
}

// Usecase contains business logic for singleplayer game management.
type Usecase struct {
	cfg    Config
	repo   Repository
	pano   PanoramaUsecase
	tracer trace.Tracer
}

// NewUsecase creates and returns a new singleplayer usecase instance with the provided dependencies.
//
// cfg - Configuration settings for the usecase.
//
// repo - Implementation of the Repository interface for accessing game and round data.
//
// pano - Implementation of the PanoramaUsecase interface for working with streetview panoramas.
func NewUsecase(cfg Config, repo Repository, pano PanoramaUsecase) *Usecase {
	return &Usecase{
		cfg:    cfg,
		repo:   repo,
		pano:   pano,
		tracer: otel.GetTracerProvider().Tracer("SingleplayerUsecase"),
	}
}
