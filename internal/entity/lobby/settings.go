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
func (s Settings) Validate() error {
	if !game.PanoramaProvider(s.Provider).IsSupported() {
		return ErrInvalidProvider
	}

	if s.Rounds < 1 || s.Rounds > 10 {
		return ErrInvalidRounds
	}

	if s.TimerSeconds != 0 && (s.TimerSeconds < 10 || s.TimerSeconds > 600) {
		return ErrInvalidTimerSeconds
	}

	return nil
}
