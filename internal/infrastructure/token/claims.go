package token

import (
	"errors"
)

// Type is a type of token (access or refresh).
type Type string

const (
	// AccessToken is a type of access token.
	AccessToken Type = "access"
	// RefreshToken is a type of refresh token.
	RefreshToken Type = "refresh"
)

// A list of keys in the claims.
const (
	ClaimsSessionIDKey string = "sessionID"
	ClaimsUserIDKey    string = "userID"
	ClaimsUsernameKey  string = "username"
	ClaimsNameKey      string = "name"
	ClaimsTokenTypeKey string = "type"
)

var (
	// ErrClaimsUserIDNotFound is returned when the userID claim is not found in the claims.
	ErrClaimsUserIDNotFound = errors.New("userID claim not found")
	// ErrClaimsUsernameNotFound is returned when the username claim is not found in the claims.
	ErrClaimsUsernameNotFound = errors.New("username claim not found")
	// ErrClaimsNameNotFound is returned when the name claim is not found in the claims.
	ErrClaimsNameNotFound = errors.New("name claim not found")
	// ErrClaimsTypeNotFound is returned when the type claim is not found in the claims.
	ErrClaimsTypeNotFound = errors.New("type claim not found")
	// ErrClaimsSessionIDNotFound is returned when the sessionID claim is not found in the claims.
	ErrClaimsSessionIDNotFound = errors.New("sessionID claim not found")
)

// GetUserID returns the user ID from the claims.
func GetUserID(claims map[string]any) (int, error) {
	id, ok := claims[ClaimsUserIDKey].(float64)
	if !ok {
		return 0, ErrClaimsUserIDNotFound
	}

	return int(id), nil
}

// GetUsername returns the username from the claims.
func GetUsername(claims map[string]any) (string, error) {
	username, ok := claims[ClaimsUsernameKey].(string)
	if !ok {
		return "", ErrClaimsUsernameNotFound
	}

	return username, nil
}

// GetName returns the name from the claims.
func GetName(claims map[string]any) (string, error) {
	name, ok := claims[ClaimsNameKey].(string)
	if !ok {
		return "", ErrClaimsNameNotFound
	}

	return name, nil
}

// GetType returns the token type from the claims.
func GetType(claims map[string]any) (Type, error) {
	typ, ok := claims[ClaimsTokenTypeKey].(string)
	if !ok {
		return "", ErrClaimsTypeNotFound
	}

	return Type(typ), nil
}

// GetSessionID returns the session ID from the claims.
func GetSessionID(claims map[string]any) (string, error) {
	sessionID, ok := claims[ClaimsSessionIDKey].(string)
	if !ok {
		return "", ErrClaimsSessionIDNotFound
	}

	return sessionID, nil
}
