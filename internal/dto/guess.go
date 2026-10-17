package dto

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/VasySS/segoya-backend/internal/entity/game"
)

// UnmarshalJSON requires an explicit guess, distinguishing null from omitted input.
func (m *MultiplayerUserGuessMessage) UnmarshalJSON(data []byte) error {
	var payload struct {
		Guess json.RawMessage `json:"guess"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode guess message: %w", err)
	}

	if len(payload.Guess) == 0 {
		return game.ErrInvalidCoordinates
	}

	if bytes.Equal(bytes.TrimSpace(payload.Guess), []byte("null")) {
		m.Guess = nil
		return nil
	}

	var coordinates struct {
		Lat *float64 `json:"lat"`
		Lng *float64 `json:"lng"`
	}

	if err := json.Unmarshal(payload.Guess, &coordinates); err != nil {
		return fmt.Errorf("decode coordinates: %w", err)
	}

	if coordinates.Lat == nil || coordinates.Lng == nil {
		return game.ErrInvalidCoordinates
	}

	guess := &game.LatLng{Lat: *coordinates.Lat, Lng: *coordinates.Lng}
	if err := guess.Validate(); err != nil {
		return fmt.Errorf("validate coordinates: %w", err)
	}

	m.Guess = guess

	return nil
}
