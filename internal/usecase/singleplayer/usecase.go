// Package singleplayer manages singleplayer game sessions including game lifecycle,
// round management, player guesses.
package singleplayer

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
	"github.com/VasySS/segoya-backend/internal/entity/playarea"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository"
)

// GameRepo defines methods for accessing and modifying singleplayer game data.
type GameRepo interface {
	InitializeSingleplayerScoring(ctx context.Context, id uuid.UUID, scoring game.ScoringSettings) error
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
	NewStreetviewInArea(
		ctx context.Context,
		provider game.PanoramaProvider,
		selection playarea.Selection,
		gameID uuid.UUID,
		multiplayer bool,
	) (game.PanoramaMetadata, error)
	GetStreetview(ctx context.Context, provider game.PanoramaProvider, id uuid.UUID) (game.PanoramaMetadata, error)
}

var errAreaServiceNotConfigured = errors.New("play area service is not configured")

// AreaUsecase resolves the selection and checks that it can supply every round.
type AreaUsecase interface {
	ResolveScoring(
		ctx context.Context,
		provider game.PanoramaProvider,
		selection playarea.Selection,
	) (game.ScoringSettings, error)
	Prepare(
		ctx context.Context,
		provider game.PanoramaProvider,
		selection playarea.Selection,
		userID uuid.UUID,
		rounds int,
	) (playarea.Selection, error)
}

// Usecase contains business logic for singleplayer game management.
type Usecase struct {
	cfg    Config
	repo   Repository
	pano   PanoramaUsecase
	areas  AreaUsecase
	tracer trace.Tracer
}

// NewUsecase creates and returns a new singleplayer usecase instance with the provided dependencies.
//
// cfg - Configuration settings for the usecase.
//
// repo - Implementation of the Repository interface for accessing game and round data.
//
// pano - Implementation of the PanoramaUsecase interface for working with streetview panoramas.
func NewUsecase(cfg Config, repo Repository, pano PanoramaUsecase, areas ...AreaUsecase) *Usecase {
	var areaUsecase AreaUsecase
	if len(areas) > 0 {
		areaUsecase = areas[0]
	}

	return &Usecase{
		areas:  areaUsecase,
		cfg:    cfg,
		repo:   repo,
		pano:   pano,
		tracer: otel.GetTracerProvider().Tracer("SingleplayerUsecase"),
	}
}
