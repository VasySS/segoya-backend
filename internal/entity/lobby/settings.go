package lobby

import "github.com/VasySS/segoya-backend/internal/entity/game"

// Settings contains the game settings of a lobby.
type Settings struct {
	Provider        string `json:"provider"`
	MovementAllowed bool   `json:"movementAllowed"`
	Rounds          int    `json:"rounds"`
	TimerSeconds    int    `json:"timerSeconds"`
}

// Validate checks the provider, round count, and timer duration in that order.
//
//nolint:wrapcheck // Domain validation returns shared sentinel errors.
func (s Settings) Validate() error {
	return game.ValidateSettings(game.PanoramaProvider(s.Provider), s.Rounds, s.TimerSeconds)
}
