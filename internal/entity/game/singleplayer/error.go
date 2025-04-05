package singleplayer

import "errors"

var (
	// ErrGameNotFound is returned when the game is not found in the database.
	ErrGameNotFound = errors.New("game not found")
	// ErrGameIsStillActive is returned when user tries to end the game that is still active (timer did not end).
	ErrGameIsStillActive = errors.New("game is still active")
	// ErrGameWrongUserID is returned when the user tries to interact with a game that does not belong to them.
	ErrGameWrongUserID = errors.New("game wrong user id")
	// ErrRoundNotFound is returned when the round is not found in the database.
	ErrRoundNotFound = errors.New("round not found")
	// ErrRoundIsStillActive is returned when user tries to end the round that is still active (timer did not end).
	ErrRoundIsStillActive = errors.New("round is still active")
	// ErrRoundMaxAmount is returned when user tries to create more rounds than allowed.
	ErrRoundMaxAmount = errors.New("round max amount")
	// ErrRoundAlreadyFinished is returned when user tries to end the round that is already finished.
	ErrRoundAlreadyFinished = errors.New("round already finished")
)
