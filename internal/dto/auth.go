package dto

import (
	"time"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// SessionsToAPI converts user sessions to struct for API response.
func SessionsToAPI(us []user.Session) *api.GetUserSessionsOKApplicationJSON {
	resp := make(api.GetUserSessionsOKApplicationJSON, 0, len(us))

	for _, session := range us {
		resp = append(resp, api.GetUserSessionsOKItem{
			SessionID:    session.SessionID,
			UserID:       session.UserID,
			RefreshToken: session.RefreshToken,
			Ua:           session.UA,
			LastActive:   session.LastActive,
		})
	}

	return &resp
}

// TokensRefreshRequest represents a request to refresh tokens.
type TokensRefreshRequest struct {
	RequestTime  time.Time
	RefreshToken string
}

// NewTokenRequest represents a request to create a new token.
type NewTokenRequest struct {
	RequestTime time.Time
	Username    string
	UserID      int
	SessionID   string
	Name        string
}

// NewSessionRequest represents a request to create a new user session.
type NewSessionRequest struct {
	RequestTime  time.Time
	UserID       int
	SessionID    string
	RefreshToken string
	UA           string
	Expiration   time.Duration
}

// UpdateSessionRequest represents a request to update an existing user session.
type UpdateSessionRequest struct {
	RequestTime  time.Time
	UserID       int
	SessionID    string
	RefreshToken string
	Expiration   time.Duration
}

// LoginRequest represents a login request with user credentials.
type LoginRequest struct {
	RequestTime time.Time
	Username    string
	Password    string
	UserAgent   string
}

// RegisterRequest represents a request to register a new user.
type RegisterRequest struct {
	RequestTime time.Time
	Username    string
	Name        string
	Password    string
}

// RegisterRequestDB represents a request to the database to register a new user.
type RegisterRequestDB struct {
	RequestTime time.Time
	Username    string
	Name        string
	Password    string
}
