// Package multiplayer contains types for working with multiplayer game data.
package multiplayer

import (
	"time"

	"github.com/VasySS/segoya-backend/internal/entity/game"
)

// Game struct contains multiplayer game information.
type Game struct {
	ID              int                   `db:"id"               json:"id"`
	CreatorID       int                   `db:"creator_id"       json:"creatorID"`
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
	ID           int       `db:"id"            json:"id"`
	GameID       int       `db:"game_id"       json:"gameID"`
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
