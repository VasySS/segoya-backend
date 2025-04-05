// Package multiplayer contains HTTP handlers for multiplayer operations.
package multiplayer

import (
	"context"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
)

// TokenService defines the interface for handling user JWT token operations.
type TokenService interface {
	FromContext(ctx context.Context) (user.AccessTokenClaims, bool)
}

// Usecase defines methods for managing multiplayer game operations.
type Usecase interface {
	GetGame(ctx context.Context, gameID, userID int) (multiplayer.Game, error)
	EndGame(ctx context.Context, req dto.EndMultiplayerGameRequest) ([]multiplayer.Guess, error)
	NewRound(ctx context.Context, req dto.NewMultiplayerRoundRequest) (multiplayer.Round, error)
	GetRound(ctx context.Context, req dto.GetMultiplayerRoundRequest) (multiplayer.Round, error)
	EndRound(ctx context.Context, req dto.EndMultiplayerRoundRequest) ([]multiplayer.Guess, error)
	NewRoundGuess(ctx context.Context, req dto.NewMultiplayerRoundGuessRequest) error
	GetGameGuesses(ctx context.Context, gameID int) ([]multiplayer.Guess, error)
	GetGameUser(ctx context.Context, userID, gameID int) (user.MultiplayerUser, error)
	GetGameUsers(ctx context.Context, gameID int) ([]user.MultiplayerUser, error)
}

var _ api.MultiplayerHandler = (*Handler)(nil)

// Handler handles HTTP requests for multiplayer operations and implements the api.MultiplayerHandler interface.
type Handler struct {
	cfg Config
	uc  Usecase
	ts  TokenService
	ws  transport.WebSocketService
}

// NewHandler creates and returns a new Handler instance with the provided dependencies.
//
// cfg - Configuration settings for the Handler.
//
// usecase - Implementation of the Usecase interface for business logic.
//
// tokenService - Implementation of the TokenService interface for handling tokens.
//
// websocketService - Implementation of the WebSocketService interface for handling WebSocket connections.
func NewHandler(
	cfg Config,
	usecase Usecase,
	tokenService TokenService,
	ws transport.WebSocketService,
) *Handler {
	h := &Handler{
		cfg: cfg,
		uc:  usecase,
		ts:  tokenService,
		ws:  ws,
	}

	h.ws.SetMessageHandler(h.handleWSMessage)
	h.ws.SetConnectHandler(h.handleWSConnect)
	h.ws.SetDisconnectHandler(h.handleWSDisconnect)

	return h
}
