// Package singleplayer contains HTTP handlers for singleplayer operations.
package singleplayer

import (
	"context"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// TokenService defines the interface for handling user JWT token operations.
type TokenService interface {
	FromContext(ctx context.Context) (user.AccessTokenClaims, bool)
}

// Usecase defines methods for managing singleplayer game operations.
type Usecase interface {
	NewGame(ctx context.Context, req dto.NewSingleplayerGameRequest) (int, error)
	GetGame(ctx context.Context, req dto.GetSingleplayerGameRequest) (singleplayer.Game, error)
	GetGames(ctx context.Context, req dto.GetSingleplayerGamesRequest) ([]singleplayer.Game, int, error)
	EndGame(ctx context.Context, req dto.EndSingleplayerGameRequest) error
	NewRound(ctx context.Context, req dto.NewSingleplayerRoundRequest) (singleplayer.Round, error)
	GetRound(ctx context.Context, req dto.GetSingleplayerRoundRequest) (singleplayer.Round, error)
	GetGameRounds(ctx context.Context, req dto.GetSingleplayerGameRoundsRequest) ([]singleplayer.Guess, error)
	EndRound(ctx context.Context, req dto.EndSingleplayerRoundRequest) (dto.EndCurrentRoundResponse, error)
}

var _ api.SingleplayerHandler = (*Handler)(nil)

// Handler handles HTTP requests for singleplayer operations and implements the api.SingleplayerHandler interface.
type Handler struct {
	cfg Config
	uc  Usecase
	ts  TokenService
}

// NewHandler creates and returns a new Handler instance with the provided dependencies.
//
// cfg - Configuration settings for the Handler.
//
// usecase - Implementation of the Usecase interface for business logic.
//
// tokenService - Implementation of the TokenService interface for handling tokens.
func NewHandler(cfg Config, usecase Usecase, ts TokenService) *Handler {
	return &Handler{
		cfg: cfg,
		uc:  usecase,
		ts:  ts,
	}
}
