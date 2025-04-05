package dto

import (
	"time"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/entity/lobby"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
)

// LobbyUserProfileKey is the key for the user profile in the WebSocket session.
const LobbyUserProfileKey string = "userProfile"

// Message types for outgoing lobby messages.
const (
	LobbyMessageError            transport.WebSocketMessageOutputType = "error"
	LobbyMessageGameRedirect     transport.WebSocketMessageOutputType = "gameRedirect"
	LobbyMessageUserConnected    transport.WebSocketMessageOutputType = "userConnected"
	LobbyMessageUserDisconnected transport.WebSocketMessageOutputType = "userDisconnected"
	LobbyMessageConnectedUsers   transport.WebSocketMessageOutputType = "usersConnected"
	LobbyMessageChatOutput       transport.WebSocketMessageOutputType = "chatMessage"
)

// Message types for incoming lobby messages.
const (
	LobbyMessageChatInput       transport.WebSocketMessageInputType = "postChatMessage"
	LobbyMessageGameStart       transport.WebSocketMessageInputType = "gameStart"
	LobbyMessageSettingsChanged transport.WebSocketMessageInputType = "settingsChanged"
)

// LobbyGameStartMessage is a message to initiate the start of the game in the lobby.
type LobbyGameStartMessage struct{}

// LobbyChatInputMessage is an incoming chat message from a user.
type LobbyChatInputMessage struct {
	Message LobbyChatMessage `json:"message"`
}

// LobbyChatMessage is a content of a chat message.
type LobbyChatMessage struct {
	Username string `json:"username"`
	Text     string `json:"text"`
}

// LobbyToAPI converts a lobby entity to its API representation.
func LobbyToAPI(l lobby.Lobby) *api.Lobby {
	return &api.Lobby{
		ID:              l.ID,
		CreatorID:       l.CreatorID,
		CreatedAt:       l.CreatedAt,
		Rounds:          l.Rounds,
		Provider:        api.Provider(l.Provider),
		MovementAllowed: l.MovementAllowed,
		TimerSeconds:    l.TimerSeconds,
		CurrentPlayers:  l.CurrentPlayers,
		MaxPlayers:      l.MaxPlayers,
	}
}

// LobbiesToAPI converts a slice of lobbies to their API representation.
func LobbiesToAPI(l []lobby.Lobby, total int) *api.LobbiesResponse {
	resp := make([]api.Lobby, 0, len(l))

	for _, v := range l {
		resp = append(resp, *LobbyToAPI(v))
	}

	return &api.LobbiesResponse{
		Total:   total,
		Lobbies: resp,
	}
}

// NewLobbyRequest is a request to create a new lobby.
type NewLobbyRequest struct {
	RequestTime     time.Time
	MaxPlayers      int
	CreatorID       int
	Rounds          int
	Provider        string
	TimerSeconds    int
	MovementAllowed bool
}

// NewLobbyRequestDB is a request to create a new lobby in the database.
type NewLobbyRequestDB struct {
	RequestTime     time.Time
	ID              string
	CreatorID       int
	Rounds          int
	Provider        string
	TimerSeconds    int
	MovementAllowed bool
	MaxPlayers      int
}

// GetLobbiesRequest is a request to get a list of lobbies.
type GetLobbiesRequest struct {
	Page     int
	PageSize int
}

// StartLobbyGameRequest is a request to start a game from a lobby.
type StartLobbyGameRequest struct {
	RequestTime      time.Time
	LobbyID          string
	Creator          user.PublicProfile
	ConnectedPlayers []user.PublicProfile
}
