package user

import "time"

// Session contains user session information.
type Session struct {
	UserID       int       `json:"userID"`
	SessionID    string    `json:"sessionID"`
	RefreshToken string    `json:"refreshToken"`
	UA           string    `json:"ua"`
	LastActive   time.Time `json:"lastActive"`
}

// AccessTokenClaims contains access token claims.
type AccessTokenClaims struct {
	SessionID string `json:"sessionID"`
	UserID    int    `json:"userID"`
	Username  string `json:"username"`
	Name      string `json:"name"`
}

// RefreshTokenClaims contains refresh token claims.
type RefreshTokenClaims struct {
	SessionID string `json:"sessionID"`
	UserID    int    `json:"userID"`
	Username  string `json:"username"`
}
