// Package lobby contains types for working with lobby data.
package lobby

import (
	"time"

	"github.com/google/uuid"
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

	return nil
}

// WithSettings returns a copy of the lobby with settings updated by its creator.
func (l Lobby) WithSettings(actorID uuid.UUID, settings Settings) (Lobby, error) {
	if l.CreatorID != actorID {
		return Lobby{}, ErrOnlyCreatorCanUpdateSettings
	}

	l.Provider = settings.Provider
	l.MovementAllowed = settings.MovementAllowed
	l.Rounds = settings.Rounds
	l.TimerSeconds = settings.TimerSeconds

	return l, nil
}
