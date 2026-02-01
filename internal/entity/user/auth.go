package user

import (
	"time"

	"github.com/google/uuid"
)

// Session contains user session information.
type Session struct {
	UserID       uuid.UUID `json:"userID"`
	SessionID    uuid.UUID `json:"sessionID"`
	RefreshToken string    `json:"refreshToken"`
	UA           string    `json:"ua"`
	LastActive   time.Time `json:"lastActive"`
}

// AccessTokenClaims contains access token claims.
type AccessTokenClaims struct {
	SessionID uuid.UUID `json:"sessionID"`
	UserID    uuid.UUID `json:"userID"`
	Username  string    `json:"username"`
	Name      string    `json:"name"`
}

// RefreshTokenClaims contains refresh token claims.
type RefreshTokenClaims struct {
	SessionID uuid.UUID `json:"sessionID"`
	UserID    uuid.UUID `json:"userID"`
	Username  string    `json:"username"`
}
