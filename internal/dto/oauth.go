package dto

import (
	"time"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// OAuthToAPI converts a slice of OAuth providers information to a format suitable for API responses.
func OAuthToAPI(oi []user.OAuth) *api.GetOAuthProvidersOKApplicationJSON {
	resp := make(api.GetOAuthProvidersOKApplicationJSON, 0, len(oi))

	for _, info := range oi {
		resp = append(resp, api.AuthProvider{
			Provider:  string(info.Issuer),
			CreatedAt: info.CreatedAt,
		})
	}

	return &resp
}

// OAuthLoginCallbackRequest represents a request to handle the OAuth login callback.
type OAuthLoginCallbackRequest struct {
	RequestTime time.Time
	Code        string
}

// NewOAuthRequest represents a request to initiate an OAuth authentication flow.
type NewOAuthRequest struct {
	RequestTime time.Time
	StateTTL    time.Duration
	State       string
	UserID      int
}

// NewOAuthCallbackRequest represents a request to handle the OAuth callback after authentication.
type NewOAuthCallbackRequest struct {
	RequestTime time.Time
	State       string
	Code        string
}

// NewOAuthRequestDB represents a request to store OAuth information in the database.
type NewOAuthRequestDB struct {
	RequestTime time.Time
	OAuthID     string
	UserID      int
	Issuer      user.OAuthIssuer
}

// DeleteOAuthRequest represents a request to delete OAuth connection.
type DeleteOAuthRequest struct {
	UserID int
	Issuer user.OAuthIssuer
}

// GetUserByOAuthRequest represents a request to get a user by their OAuth connection.
type GetUserByOAuthRequest struct {
	OAuthID string
	Issuer  user.OAuthIssuer
}
