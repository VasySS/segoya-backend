// Package lobby contains types for working with lobby data.
package lobby

import (
	"time"

	"github.com/google/uuid"

	"github.com/VasySS/segoya-backend/internal/entity/game"
)

// Lobby struct contains lobby information, including game details.
type Lobby struct {
	ID              string    `json:"id"`
	CreatorID       uuid.UUID `json:"creatorID"`
	CreatedAt       time.Time `json:"createdAt"`
	Rounds          int       `json:"rounds"`
	Provider        string    `json:"provider"`
	MovementAllowed bool      `json:"movementAllowed"`
	TimerSeconds    int       `json:"timerSeconds"`
	CurrentPlayers  int       `json:"currentPlayers"`
	MaxPlayers      int       `json:"maxPlayers"`
	Private         bool      `json:"private"`
}

// IsFull returns true if the lobby has reached its maximum player capacity.
func (l Lobby) IsFull() bool {
	return l.CurrentPlayers >= l.MaxPlayers
}

// IsLastPlayer reports whether the next departure will leave the lobby empty.
func (l Lobby) IsLastPlayer() bool {
	return l.CurrentPlayers == 1
}

// ValidateStart checks whether the actor can start a game from this lobby.
func (l Lobby) ValidateStart(actorID uuid.UUID) error {
	if l.CreatorID != actorID {
		return ErrOnlyCreatorCanStart
	}

	if err := l.Validate(); err != nil {
		return err
	}

	if l.CurrentPlayers < 2 {
		return ErrInvalidPlayers
	}

	return nil
}

// WithSettings returns a copy of the lobby with settings updated by its creator.
func (l Lobby) WithSettings(actorID uuid.UUID, settings Settings) (Lobby, error) {
	if l.CreatorID != actorID {
		return Lobby{}, ErrOnlyCreatorCanUpdateSettings
	}

	if err := settings.Validate(); err != nil {
		return Lobby{}, err
	}

	l.Provider = settings.Provider
	l.MovementAllowed = settings.MovementAllowed
	l.Rounds = settings.Rounds
	l.TimerSeconds = settings.TimerSeconds

	return l, nil
}

// Validate checks settings and lobby capacity, including persisted player counts.
//
//nolint:wrapcheck // Domain validation returns shared sentinel errors.
func (l Lobby) Validate() error {
	if err := game.ValidateSettings(game.PanoramaProvider(l.Provider), l.Rounds, l.TimerSeconds); err != nil {
		return err
	}

	if l.MaxPlayers < 2 || l.MaxPlayers > 10 || l.CurrentPlayers < 0 || l.CurrentPlayers > l.MaxPlayers {
		return ErrInvalidPlayers
	}

	return nil
}
