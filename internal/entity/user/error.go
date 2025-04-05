package user

import "errors"

var (
	// ErrSessionNotFound is returned when the session is not found in the database.
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionWrongUser is returned when the user tries to interact with a session that does not belong to them.
	ErrSessionWrongUser = errors.New("session belongs to another user")
	// ErrOAuthNotFound is returned when the oauth connection for user is not found in the database.
	ErrOAuthNotFound = errors.New("oauth is not connected")
	// ErrOAuthAlreadyExists is returned when the user tries to connect an oauth that is already connected to another user.
	ErrOAuthAlreadyExists = errors.New("oauth is already connected to another user")
	// ErrUserNotFound is returned when the user tries to access a user that does not exist.
	ErrUserNotFound = errors.New("user not found")
	// ErrAlreadyExists is returned when the user tries to create an aco=count that already exists.
	ErrAlreadyExists = errors.New("user already exists")
	// ErrWrongPassword is returned when the user tries to login with a wrong password.
	ErrWrongPassword = errors.New("wrong password")
	// ErrWrongTokenType is returned when the user tries to do something with a token of a wrong type -
	// access instead of refresh or vice versa.
	ErrWrongTokenType = errors.New("wrong token type")
	// ErrAvatarUpdateTooFrequent is returned when the user tries to update avatar too often.
	ErrAvatarUpdateTooFrequent = errors.New("avatar update too frequent")
)
