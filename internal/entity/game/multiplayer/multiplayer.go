// Package multiplayer contains types for working with multiplayer game data.
package multiplayer

import (
	"time"

	"github.com/google/uuid"

	"github.com/VasySS/segoya-backend/internal/entity/game"
)

// MaxRoundsReached returns true if all rounds have been created.
func (g Game) MaxRoundsReached() bool {
	return g.RoundCurrent != 0 && g.RoundCurrent == g.Rounds
}

// IsComplete returns true if the game is fully finished (all rounds done).
func (g Game) IsComplete() bool {
	return g.RoundCurrent >= g.Rounds && g.Finished
}

// TimerPassed returns true if there is no timer or the timer has expired.
func (r Round) TimerPassed(now time.Time, timerSeconds int) bool {
	return timerSeconds == 0 || !now.Before(r.StartedAt.Add(time.Second*time.Duration(timerSeconds)))
}

// AllPlayersGuessed returns true if all players have submitted a guess for this round.
func (r Round) AllPlayersGuessed(players int) bool {
	return r.GuessesCount == players
}

// Game struct contains multiplayer game information.
type Game struct {
	ID              uuid.UUID             `db:"id"               json:"id"`
	CreatorID       uuid.UUID             `db:"creator_id"       json:"creatorID"`
	Rounds          int                   `db:"rounds"           json:"rounds"`
	RoundCurrent    int                   `db:"round_current"    json:"roundCurrent"`
	MovementAllowed bool                  `db:"movement_allowed" json:"movementAllowed"`
	Provider        game.PanoramaProvider `db:"provider"         json:"provider"`
	TimerSeconds    int                   `db:"timer_seconds"    json:"timerSeconds"`
	Players         int                   `db:"players"          json:"players"`
	Finished        bool                  `db:"finished"         json:"finished"`
	CreatedAt       time.Time             `db:"created_at"       json:"createdAt"`
	EndedAt         time.Time             `db:"ended_at"         json:"endedAt"`
}

// Round struct contains multiplayer round information.
type Round struct {
	ID           uuid.UUID `db:"id"            json:"id"`
	GameID       uuid.UUID `db:"game_id"       json:"gameID"`
	RoundNum     int       `db:"round_num"     json:"roundNum"`
	StreetviewID string    `db:"streetview_id" json:"streetviewID"`
	Lat          float64   `db:"lat"           json:"lat"`
	Lng          float64   `db:"lng"           json:"lng"`
	GuessesCount int       `db:"guesses_count" json:"guessesCount"`
	Finished     bool      `db:"finished"      json:"finished"`
	CreatedAt    time.Time `db:"created_at"    json:"createdAt"`
	StartedAt    time.Time `db:"started_at"    json:"startedAt"`
	EndedAt      time.Time `db:"ended_at"      json:"endedAt"`
}

// Guess struct contains multiplayer user's guess information.
type Guess struct {
	Username   string  `json:"username"`
	AvatarHash string  `json:"avatarHash"`
	RoundNum   int     `json:"roundNum"`
	RoundLat   float64 `json:"roundLat"`
	RoundLng   float64 `json:"roundLng"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	Score      int     `json:"score"`
}
