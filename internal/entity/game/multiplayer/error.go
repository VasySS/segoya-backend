package multiplayer

import "errors"

var (
	// ErrGameNotFound is returned when the game is not found in the database.
	ErrGameNotFound = errors.New("game not found")
	// ErrGameIsStillActive is returned when user tries to end the game that is still active -
	// current round is not finished and/or current round is not the last one.
	ErrGameIsStillActive = errors.New("game is still active")
	// ErrGameWrongUserID is returned when the user tries to interact with a game they are not a part of.
	ErrGameWrongUserID = errors.New("game wrong user id")
	// ErrRoundNotFound is returned when the round is not found in the database.
	ErrRoundNotFound = errors.New("round not found")
	// ErrRoundIsStillActive is returned when user tries to end the round that is still active -
	// timer did not end and/or not everyone sent their guess.
	ErrRoundIsStillActive = errors.New("round is still active")
	// ErrRoundMaxAmount is returned when user tries to create more rounds than allowed.
	ErrRoundMaxAmount = errors.New("round max amount")
	// ErrRoundAlreadyFinished is returned when user tries to send guess after round has ended.
	ErrRoundAlreadyFinished = errors.New("round already finished")
)
