package game

import (
	"time"

	"github.com/google/uuid"
)

// RoundCreation contains the values needed to create a game round.
type RoundCreation struct {
	GameID     uuid.UUID
	LocationID uuid.UUID
	RoundNum   int
	CreatedAt  time.Time
	StartedAt  time.Time
}
