// Package singleplayer contains types for working with singleplayer game data.
package singleplayer

import (
	"time"

	"github.com/google/uuid"

	"github.com/VasySS/segoya-backend/internal/entity/game"
)

// MaxRoundsReached returns true if all rounds have been created.
func (g Game) MaxRoundsReached() bool {
	return g.RoundCurrent != 0 && g.RoundCurrent == g.Rounds
}

// AllRoundsCreated returns true if all rounds have been created.
func (g Game) AllRoundsCreated() bool {
	return g.RoundCurrent == g.Rounds
}

// TimerExpired returns true if the timer is enabled and has expired.
func (r Round) TimerExpired(now time.Time, timerSeconds int) bool {
	return timerSeconds != 0 && now.After(r.StartedAt.Add(time.Second*time.Duration(timerSeconds)))
}

// Game struct contains singleplayer game information.
type Game struct {
	ID              uuid.UUID             `db:"id"               json:"id"`
	UserID          uuid.UUID             `db:"user_id"          json:"userID"`
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

// HasUser checks that the game belongs to the user.
func (g Game) HasUser(userID uuid.UUID) error {
	if g.UserID != userID {
		return ErrGameWrongUserID
	}

	return nil
}

// ValidateNewRound checks that another round can be created.
func (g Game) ValidateNewRound() error {
	if g.MaxRoundsReached() {
		return ErrRoundMaxAmount
	}

	return nil
}

// ValidateEnd checks that all rounds have been created before ending the game.
func (g Game) ValidateEnd() error {
	if !g.AllRoundsCreated() {
		return ErrGameIsStillActive
	}

	return nil
}

// ValidateGuessesAccess checks that the game has finished before showing its guesses.
func (g Game) ValidateGuessesAccess() error {
	if !g.Finished {
		return ErrGameIsStillActive
	}

	return nil
}

// PrepareNextRound returns the creation values for the next round.
func (g Game) PrepareNextRound(
	panorama game.PanoramaMetadata,
	requestTime time.Time,
	startDelay time.Duration,
) game.RoundCreation {
	return game.RoundCreation{
		GameID:     g.ID,
		LocationID: panorama.ID,
		RoundNum:   g.RoundCurrent + 1,
		CreatedAt:  requestTime,
		StartedAt:  requestTime.Add(startDelay),
	}
}

// Round struct contains singleplayer round information.
type Round struct {
	ID           uuid.UUID `db:"id"            json:"id"`
	GameID       uuid.UUID `db:"game_id"       json:"gameID"`
	StreetviewID string    `db:"streetview_id" json:"streetviewID"`
	Lat          float64   `db:"lat"           json:"lat"`
	Lng          float64   `db:"lng"           json:"lng"`
	RoundNum     int       `db:"round_num"     json:"roundNum"`
	Finished     bool      `db:"finished"      json:"finished"`
	CreatedAt    time.Time `db:"created_at"    json:"createdAt"`
	StartedAt    time.Time `db:"started_at"    json:"startedAt"`
	EndedAt      time.Time `db:"ended_at"      json:"endedAt"`
}

// CanReuse returns true if the round is unfinished.
func (r Round) CanReuse() bool {
	return !r.Finished
}

// ValidateFinished checks that the round has finished before ending its game.
func (r Round) ValidateFinished() error {
	if !r.Finished {
		return ErrRoundIsStillActive
	}

	return nil
}

// EvaluateGuess returns the score and distance, applying the penalty for a late guess.
func (r Round) EvaluateGuess(g Game, requestTime time.Time, coordinates game.LatLng) (int, int, error) {
	if r.Finished {
		return 0, 0, ErrRoundAlreadyFinished
	}

	location := game.LatLng{Lat: r.Lat, Lng: r.Lng}

	score, distance := game.CalculateScore(g.Provider, location, coordinates)
	if r.TimerExpired(requestTime, g.TimerSeconds) {
		score = 0
	}

	return score, distance, nil
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
