package token

import (
	"fmt"

	"github.com/lestrrat-go/jwx/v3/jwt"
)

// Type is a type of JWT token (access or refresh).
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

// GetUserID returns the user ID from the claims.
func GetUserID(token jwt.Token) (int, error) {
	var userID float64

	if err := token.Get(ClaimsUserIDKey, &userID); err != nil {
		return 0, fmt.Errorf("error getting userID from claims: %w", err)
	}

	return int(userID), nil
}

// GetUsername returns the username from the claims.
func GetUsername(token jwt.Token) (string, error) {
	var username string

	if err := token.Get(ClaimsUsernameKey, &username); err != nil {
		return "", fmt.Errorf("error getting username from claims: %w", err)
	}

	return username, nil
}

// GetName returns the name from the claims.
func GetName(token jwt.Token) (string, error) {
	var name string

	if err := token.Get(ClaimsNameKey, &name); err != nil {
		return "", fmt.Errorf("error getting name from claims: %w", err)
	}

	return name, nil
}

// GetType returns the token type from the claims.
func GetType(token jwt.Token) (Type, error) {
	var typ string

	if err := token.Get(ClaimsTokenTypeKey, &typ); err != nil {
		return "", fmt.Errorf("error getting type from claims: %w", err)
	}

	return Type(typ), nil
}

// GetSessionID returns the session ID from the claims.
func GetSessionID(token jwt.Token) (string, error) {
	var sessionID string

	if err := token.Get(ClaimsSessionIDKey, &sessionID); err != nil {
		return "", fmt.Errorf("error getting sessionID from claims: %w", err)
	}

	return sessionID, nil
}
