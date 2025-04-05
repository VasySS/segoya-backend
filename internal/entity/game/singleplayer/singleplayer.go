// Package singleplayer contains types for working with singleplayer game data.
package singleplayer

import (
	"time"

	"github.com/VasySS/segoya-backend/internal/entity/game"
)

// Game struct contains singleplayer game information.
type Game struct {
	ID              int                   `db:"id"               json:"id"`
	UserID          int                   `db:"user_id"          json:"userID"`
	Rounds          int                   `db:"rounds"           json:"rounds"`
	RoundCurrent    int                   `db:"round_current"    json:"roundCurrent"`
	TimerSeconds    int                   `db:"timer_seconds"    json:"timerSeconds"`
	MovementAllowed bool                  `db:"movement_allowed" json:"movementAllowed"`
	Provider        game.PanoramaProvider `db:"provider"         json:"provider"`
	Score           int                   `db:"score"            json:"score"`
	Finished        bool                  `db:"finished"         json:"finished"`
	CreatedAt       time.Time             `db:"created_at"       json:"createdAt"`
	EndedAt         time.Time             `db:"ended_at"         json:"endedAt"`
}

// Round struct contains singleplayer round information.
type Round struct {
	ID           int       `db:"id"            json:"id"`
	GameID       int       `db:"game_id"       json:"gameID"`
	StreetviewID string    `db:"streetview_id" json:"streetviewID"`
	Lat          float64   `db:"lat"           json:"lat"`
	Lng          float64   `db:"lng"           json:"lng"`
	RoundNum     int       `db:"round_num"     json:"roundNum"`
	Finished     bool      `db:"finished"      json:"finished"`
	CreatedAt    time.Time `db:"created_at"    json:"createdAt"`
	StartedAt    time.Time `db:"started_at"    json:"startedAt"`
	EndedAt      time.Time `db:"ended_at"      json:"endedAt"`
}

// Guess struct contains singleplayer guess information.
type Guess struct {
	RoundNum     int     `json:"roundNum"`
	RoundLat     float64 `json:"roundLat"`
	RoundLng     float64 `json:"roundLng"`
	GuessLat     float64 `json:"guessLat"`
	GuessLng     float64 `json:"guessLng"`
	Score        int     `json:"score"`
	MissDistance int     `json:"missDistance"`
}
