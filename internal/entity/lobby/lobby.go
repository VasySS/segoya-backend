// Package lobby contains types for working with lobby data.
package lobby

import (
	"time"
)

// Lobby struct contains lobby information, including game details.
type Lobby struct {
	ID              string    `json:"id"`
	CreatorID       int       `json:"creatorID"`
	CreatedAt       time.Time `json:"createdAt"`
	Rounds          int       `json:"rounds"`
	Provider        string    `json:"provider"`
	MovementAllowed bool      `json:"movementAllowed"`
	TimerSeconds    int       `json:"timerSeconds"`
	CurrentPlayers  int       `json:"currentPlayers"`
	MaxPlayers      int       `json:"maxPlayers"`
}
