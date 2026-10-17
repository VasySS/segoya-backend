package game

import "errors"

var (
	ErrInvalidProvider     = errors.New("invalid panorama provider")
	ErrInvalidRounds       = errors.New("invalid rounds")
	ErrInvalidTimerSeconds = errors.New("invalid timer seconds")
	ErrInvalidPlayers      = errors.New("invalid players")
	ErrInvalidState        = errors.New("invalid game state")
	ErrInvalidCoordinates  = errors.New("invalid coordinates")
	ErrRoundNotStarted     = errors.New("round has not started")
)

// ValidateSettings checks provider, rounds, and timer in transport-compatible order.
func ValidateSettings(provider PanoramaProvider, rounds, timerSeconds int) error {
	if !provider.IsSupported() {
		return ErrInvalidProvider
	}

	if rounds < 1 || rounds > 10 {
		return ErrInvalidRounds
	}

	if timerSeconds != 0 && (timerSeconds < 10 || timerSeconds > 600) {
		return ErrInvalidTimerSeconds
	}

	return nil
}

// ValidateRoundState rejects impossible counts, including incomplete finished games.
func ValidateRoundState(rounds, current int, finished bool) error {
	if rounds < 1 || rounds > 10 {
		return ErrInvalidRounds
	}

	if current < 0 || current > rounds || (finished && current != rounds) {
		return ErrInvalidState
	}

	return nil
}
