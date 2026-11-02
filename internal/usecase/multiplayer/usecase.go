// Package multiplayer manages multiplayer game sessions including game lifecycle,
// round management, player guesses.
package multiplayer

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/playarea"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository"
)

// GameRepo provides access to multiplayer game data.
type GameRepo interface {
	InitializeMultiplayerScoring(ctx context.Context, id uuid.UUID, scoring game.ScoringSettings) error
	LockMultiplayerGame(ctx context.Context, gameID uuid.UUID) error
	NewMultiplayerGame(ctx context.Context, req dto.NewMultiplayerGameRequest) (uuid.UUID, error)
	GetMultiplayerGame(ctx context.Context, gameID uuid.UUID) (multiplayer.Game, error)
	EndMultiplayerGame(ctx context.Context, req dto.EndMultiplayerGameRequestDB) error
	GetMultiplayerGameUser(ctx context.Context, userID, gameID uuid.UUID) (user.MultiplayerUser, error)
	GetMultiplayerGameUsers(ctx context.Context, gameID uuid.UUID) ([]user.MultiplayerUser, error)
	GetMultiplayerGameGuesses(ctx context.Context, gameID uuid.UUID) ([]multiplayer.Guess, error)
}

// RoundRepo provides access to multiplayer round data.
type RoundRepo interface {
	NewMultiplayerRound(ctx context.Context, req dto.NewMultiplayerRoundRequestDB) (multiplayer.Round, error)
	GetMultiplayerRound(ctx context.Context, gameID uuid.UUID, roundNum int) (multiplayer.Round, error)
	EndMultiplayerRound(ctx context.Context, req dto.EndMultiplayerRoundRequestDB) error
	NewMultiplayerRoundGuess(ctx context.Context, req dto.NewMultiplayerRoundGuessRequestDB) error
	GetMultiplayerRoundGuesses(ctx context.Context, roundID uuid.UUID) ([]multiplayer.Guess, error)
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

// Usecase contains business logic for multiplayer game management.
type Usecase struct {
	cfg    Config
	repo   Repository
	pano   PanoramaUsecase
	areas  AreaUsecase
	tracer trace.Tracer
}

// NewUsecase creates and returns a new instance of multiplayer usecase with the provided dependencies.
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
		tracer: otel.GetTracerProvider().Tracer("MultiplayerUsecase"),
	}
}
