package dto

import (
	"time"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
)

// MultiplayerUserProfileKey is the key for the user profile in the WebSocket session.
const MultiplayerUserProfileKey string = "userProfile"

// Message types for outgoing multiplayer messages.
const (
	MultiplayerMessageError            transport.WebSocketMessageOutputType = "error"
	MultiplayerMessageUserConnected    transport.WebSocketMessageOutputType = "userConnected"
	MultiplayerMessageUserDisconnected transport.WebSocketMessageOutputType = "userDisconnected"
	MultiplayerMessageConnectedUsers   transport.WebSocketMessageOutputType = "usersConnected"
	MultiplayerMessageUserGuessed      transport.WebSocketMessageOutputType = "userGuessed"
	MultiplayerMessageGameFinished     transport.WebSocketMessageOutputType = "gameFinished"
	MultiplayerMessageRoundFinished    transport.WebSocketMessageOutputType = "roundFinished"
)

// Message types for incoming multiplayer messages.
const (
	MultiplayerMessageUserGuess transport.WebSocketMessageInputType = "userGuess"
	MultiplayerMessageRoundEnd  transport.WebSocketMessageInputType = "endRound"
)

// MultiplayerUserGuessMessage is an incoming message with a user guess.
type MultiplayerUserGuessMessage struct {
	Guess game.LatLng `json:"guess"`
}

// MultiplayerGameToAPI converts a multiplayer game entity to the API model.
func MultiplayerGameToAPI(g multiplayer.Game) *api.MultiplayerGame {
	return &api.MultiplayerGame{
		ID:              g.ID,
		CreatorID:       g.CreatorID,
		Rounds:          g.Rounds,
		RoundCurrent:    g.RoundCurrent,
		MovementAllowed: g.MovementAllowed,
		Provider:        api.Provider(g.Provider),
		TimerSeconds:    g.TimerSeconds,
		Players:         g.Players,
		Finished:        g.Finished,
		CreatedAt:       g.CreatedAt,
	}
}

// MultiplayerRoundToAPI converts a multiplayer round entity to the API model.
func MultiplayerRoundToAPI(r multiplayer.Round) *api.MultiplayerRound {
	return &api.MultiplayerRound{
		ID:           r.ID,
		GameID:       r.GameID,
		StreetviewID: r.StreetviewID,
		RoundNum:     r.RoundNum,
		Lat:          r.Lat,
		Lng:          r.Lng,
		GuessesCount: r.GuessesCount,
		CreatedAt:    r.CreatedAt,
		StartedAt:    r.StartedAt,
		Finished:     r.Finished,
		EndedAt:      r.EndedAt,
	}
}

// MultiplayerGameGuessesToAPI converts a slice of multiplayer guess entities to the API model.
func MultiplayerGameGuessesToAPI(guesses []multiplayer.Guess) *api.GetMultiplayerGameGuessesOKApplicationJSON {
	resp := make(api.GetMultiplayerGameGuessesOKApplicationJSON, 0, len(guesses))

	for _, g := range guesses {
		resp = append(resp, api.MultiplayerGuess{
			Username:   g.Username,
			AvatarHash: g.AvatarHash,
			RoundNum:   g.RoundNum,
			RoundLat:   g.RoundLat,
			RoundLng:   g.RoundLng,
			Lat:        g.Lat,
			Lng:        g.Lng,
			Score:      g.Score,
		})
	}

	return &resp
}

// NewMultiplayerGameRequest is a request for a new multiplayer game.
type NewMultiplayerGameRequest struct {
	RequestTime      time.Time
	CreatorID        int
	ConnectedPlayers []user.PublicProfile
	Rounds           int
	TimerSeconds     int
	MovementAllowed  bool
	Provider         string
}

// EndMultiplayerGameRequestDB is a request to end a multiplayer game in the database.
type EndMultiplayerGameRequestDB struct {
	RequestTime time.Time
	GameID      int
}

// NewMultiplayerRoundGuessRequest is a request to create a new multiplayer round guess.
type NewMultiplayerRoundGuessRequest struct {
	RequestTime time.Time
	UserID      int
	GameID      int
	Guess       game.LatLng
}

// NewMultiplayerRoundGuessRequestDB is a request to create a new multiplayer round guess in the database.
type NewMultiplayerRoundGuessRequestDB struct {
	RequestTime time.Time
	UserID      int
	RoundID     int
	Lat         float64
	Lng         float64
	Score       int
	Distance    int
}

// GetMultiplayerRoundRequest is a request to get a multiplayer round.
type GetMultiplayerRoundRequest struct {
	RequestTime time.Time
	GameID      int
	UserID      int
}

// NewMultiplayerRoundRequest is a request to create a new multiplayer round.
type NewMultiplayerRoundRequest struct {
	RequestTime time.Time
	GameID      int
	UserID      int
}

// NewMultiplayerRoundRequestDB is a request to create a new multiplayer round in the database.
type NewMultiplayerRoundRequestDB struct {
	GameID     int
	LocationID int
	RoundNum   int
	CreatedAt  time.Time
	StartedAt  time.Time
}

// EndMultiplayerRoundRequest is a request to end a multiplayer round.
type EndMultiplayerRoundRequest struct {
	RequestTime time.Time
	GameID      int
	UserID      int
}

// EndMultiplayerRoundRequestDB is a request to end a multiplayer round in the database.
type EndMultiplayerRoundRequestDB struct {
	RequestTime time.Time
	RoundID     int
}

// EndMultiplayerGameRequest is a request to end a multiplayer game.
type EndMultiplayerGameRequest struct {
	RequestTime time.Time
	GameID      int
	UserID      int
}
