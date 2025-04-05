package dto

import (
	"time"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
)

// SingleplayerGameToAPI converts a singleplayer game entity to the API model.
func SingleplayerGameToAPI(g singleplayer.Game) *api.SingleplayerGame {
	return &api.SingleplayerGame{
		ID:              g.ID,
		UserID:          g.UserID,
		Rounds:          g.Rounds,
		RoundCurrent:    g.RoundCurrent,
		TimerSeconds:    g.TimerSeconds,
		MovementAllowed: g.MovementAllowed,
		Provider:        api.Provider(g.Provider),
		Score:           g.Score,
		Finished:        g.Finished,
		CreatedAt:       g.CreatedAt,
	}
}

// SingleplayerRoundToAPI converts a singleplayer round entity to the API model.
func SingleplayerRoundToAPI(r singleplayer.Round) *api.SingleplayerRoundResp {
	return &api.SingleplayerRoundResp{
		ID:           r.ID,
		GameID:       r.GameID,
		StreetviewID: r.StreetviewID,
		RoundNum:     r.RoundNum,
		Lat:          r.Lat,
		Lng:          r.Lng,
		Finished:     r.Finished,
		CreatedAt:    r.CreatedAt,
		StartedAt:    r.StartedAt,
	}
}

// SingleplayerRoundResultToAPI converts a singleplayer round result entity to the API model.
func SingleplayerRoundResultToAPI(r EndCurrentRoundResponse) *api.EndSingleplayerRoundResp {
	return &api.EndSingleplayerRoundResp{
		Score:    r.Score,
		Distance: r.Distance,
	}
}

// SingleplayerRoundsToAPI converts a slice of singleplayer round entities to the API model.
func SingleplayerRoundsToAPI(rounds []singleplayer.Guess) *api.GetSingleplayerGameRoundsOKApplicationJSON {
	resp := make(api.GetSingleplayerGameRoundsOKApplicationJSON, 0, len(rounds))

	for _, r := range rounds {
		resp = append(resp, api.SingleplayerRoundsWithGuess{
			RoundNum:     r.RoundNum,
			RoundLat:     r.RoundLat,
			RoundLng:     r.RoundLng,
			GuessLat:     r.GuessLat,
			GuessLng:     r.GuessLng,
			Score:        r.Score,
			MissDistance: r.MissDistance,
		})
	}

	return &resp
}

// SingleplyerGamesToAPI converts a slice of singleplayer game entities to the API model.
func SingleplyerGamesToAPI(g []singleplayer.Game, gamesTotal int) *api.SingleplayerGames {
	games := make([]api.SingleplayerGame, 0, len(g))

	for _, game := range g {
		games = append(games, api.SingleplayerGame{
			ID:              game.ID,
			Rounds:          game.Rounds,
			UserID:          game.UserID,
			RoundCurrent:    game.RoundCurrent,
			TimerSeconds:    game.TimerSeconds,
			MovementAllowed: game.MovementAllowed,
			Provider:        api.Provider(game.Provider),
			Score:           game.Score,
			Finished:        game.Finished,
			CreatedAt:       game.CreatedAt,
		})
	}

	return &api.SingleplayerGames{
		Total: gamesTotal,
		Games: games,
	}
}

// GetSingleplayerGamesRequest represents a request to get a list of singleplayer games.
type GetSingleplayerGamesRequest struct {
	UserID   int
	Page     int
	PageSize int
}

// NewSingleplayerRoundDBRequest represents a request to create a new singleplayer round.
type NewSingleplayerRoundDBRequest struct {
	CreatedAt  time.Time
	StartedAt  time.Time
	GameID     int
	LocationID int
	RoundNum   int
}

// NewSingleplayerGameRequest represents a request to create a new singleplayer game.
type NewSingleplayerGameRequest struct {
	RequestTime     time.Time
	UserID          int
	Rounds          int
	TimerSeconds    int
	Provider        string
	MovementAllowed bool
}

// EndSingleplayerGameRequestDB represents a request to end a singleplayer game.
type EndSingleplayerGameRequestDB struct {
	RequestTime time.Time
	GameID      int
}

// EndSingleplayerRoundRequest represents a request to end a singleplayer round.
type EndSingleplayerRoundRequest struct {
	RequestTime time.Time
	GameID      int
	UserID      int
	Guess       game.LatLng
}

// NewSingleplayerRoundGuessRequest represents a request to create a new singleplayer round guess.
type NewSingleplayerRoundGuessRequest struct {
	RequestTime time.Time
	RoundID     int
	GameID      int
	Guess       game.LatLng
	Score       int
	Distance    int
}

// EndCurrentRoundResponse represents a response to end a singleplayer round.
type EndCurrentRoundResponse struct {
	Score    int
	Distance int
}

// GetSingleplayerGameRequest represents a request to get a singleplayer game.
type GetSingleplayerGameRequest struct {
	RequestTime time.Time
	GameID      int
	UserID      int
}

// GetSingleplayerRoundRequest represents a request to get a singleplayer round.
type GetSingleplayerRoundRequest struct {
	RequestTime time.Time
	GameID      int
	UserID      int
}

// GetSingleplayerGameRoundsRequest represents a request to get a list of singleplayer game rounds.
type GetSingleplayerGameRoundsRequest struct {
	RequestTime time.Time
	GameID      int
	UserID      int
}

// NewSingleplayerRoundRequest represents a request to create a new singleplayer round.
type NewSingleplayerRoundRequest struct {
	RequestTime time.Time
	GameID      int
	UserID      int
}

// EndSingleplayerGameRequest represents a request to end a singleplayer game.
type EndSingleplayerGameRequest struct {
	RequestTime time.Time
	GameID      int
	UserID      int
}
