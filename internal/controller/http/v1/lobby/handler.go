// Package lobby contains HTTP handlers for lobby operations.
package lobby

import (
	"context"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/lobby"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
)

// TokenService defines the interface for handling user JWT token operations.
type TokenService interface {
	FromContext(ctx context.Context) (user.AccessTokenClaims, bool)
}

// Usecase defines methods for managing lobby operations.
type Usecase interface {
	NewLobby(ctx context.Context, req dto.NewLobbyRequest) (string, error)
	GetLobby(ctx context.Context, id string) (lobby.Lobby, error)
	DeleteLobby(ctx context.Context, id string) error
	GetLobbies(ctx context.Context, req dto.GetLobbiesRequest) ([]lobby.Lobby, int, error)
	ConnectLobbyUser(ctx context.Context, lobbyID string, userID int) (user.PublicProfile, error)
	DisconnectLobbyUser(ctx context.Context, lobbyID string, userID int) error
	StartLobbyGame(ctx context.Context, req dto.StartLobbyGameRequest) (int, error)
}

var _ api.LobbiesHandler = (*Handler)(nil)

// Handler implements the api.LobbiesHandler interface and handles HTTP requests for lobby operations.
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
	websocketService transport.WebSocketService,
) *Handler {
	h := &Handler{
		cfg: cfg,
		uc:  usecase,
		ts:  tokenService,
		ws:  websocketService,
	}

	h.ws.SetMessageHandler(h.handleWSMessage)
	h.ws.SetConnectHandler(h.handleWSConnect)
	h.ws.SetDisconnectHandler(h.handleWSDisconnect)

	return h
}
