package lobby

import (
	"errors"

	"github.com/VasySS/segoya-backend/internal/entity/game"
)

var (
	// ErrNotFound is returned when the lobby is not found in the database.
	ErrNotFound = errors.New("lobby not found")
	// ErrOnlyCreatorCanStart is returned when the user tries to start the game
	// from lobby that was not created by them.
	ErrOnlyCreatorCanStart = errors.New("only creator can start the game")
	// ErrOnlyCreatorCanUpdateSettings is returned when the user tries to update
	// settings of the lobby that was not created by them.
	ErrOnlyCreatorCanUpdateSettings = errors.New("only creator can update settings")
	// ErrLobbyIsFull is returned when user tries to join a lobby that is full.
	ErrLobbyIsFull = errors.New("lobby is full")
	// ErrInvalidProvider is returned when settings use an unsupported panorama provider.
	ErrInvalidProvider = game.ErrInvalidProvider
	// ErrInvalidRounds is returned when settings specify an invalid round count.
	ErrInvalidRounds = game.ErrInvalidRounds
	// ErrInvalidTimerSeconds is returned when settings specify an invalid timer duration.
	ErrInvalidTimerSeconds = game.ErrInvalidTimerSeconds
	ErrInvalidPlayers      = game.ErrInvalidPlayers
	ErrAlreadyConnected    = errors.New("user already connected to lobby")
)
