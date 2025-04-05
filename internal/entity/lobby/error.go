package lobby

import "errors"

var (
	// ErrNotFound is returned when the lobby is not found in the database.
	ErrNotFound = errors.New("lobby not found")
	// ErrOnlyCreatorCanStart is returned when the user tries to start the game
	// from lobby that was not created by them.
	ErrOnlyCreatorCanStart = errors.New("only creator can start the game")
	// ErrLobbyIsFull is returned when user tries to join a lobby that is full.
	ErrLobbyIsFull = errors.New("lobby is full")
)
