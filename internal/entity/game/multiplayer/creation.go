package multiplayer

import (
	"github.com/google/uuid"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// ValidateParticipants requires a unique, nonempty identity for every player and the creator.
func ValidateParticipants(creatorID uuid.UUID, players []user.PublicProfile) error {
	if len(players) < 2 || len(players) > 10 {
		return game.ErrInvalidPlayers
	}

	seen := make(map[uuid.UUID]bool, len(players))
	for _, player := range players {
		if player.ID == uuid.Nil || seen[player.ID] {
			return game.ErrInvalidPlayers
		}

		seen[player.ID] = true
	}

	if !seen[creatorID] {
		return game.ErrInvalidPlayers
	}

	return nil
}
