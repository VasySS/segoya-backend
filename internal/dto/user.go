package dto

import (
	"io"
	"time"

	api "github.com/VasySS/segoya-backend/api/ogen"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

// UserToAPIPublicUser converts a user profile to a API format.
func UserToAPIPublicUser(u user.PublicProfile) *api.UserPublicProfile {
	return &api.UserPublicProfile{
		ID:           u.ID,
		Username:     u.Username,
		Name:         u.Name,
		AvatarHash:   u.AvatarHash,
		RegisterDate: u.RegisterDate,
	}
}

// UserToAPIPrivateUser converts a user profile to a API format.
func UserToAPIPrivateUser(u user.PrivateProfile) *api.UserPrivateProfile {
	return &api.UserPrivateProfile{
		ID:           u.ID,
		Username:     u.Username,
		Name:         u.Name,
		AvatarHash:   u.AvatarHash,
		RegisterDate: u.RegisterDate,
	}
}

// UpdateUserRequest represents a request to update a user's profile information.
type UpdateUserRequest struct {
	UserID int
	Name   string
}

// UpdateAvatarRequest represents a request to update a user's avatar.
type UpdateAvatarRequest struct {
	RequestTime time.Time
	UserID      int
	File        io.Reader
	MimeType    string
}

// UpdateAvatarRequestDB represents a request to update a user's avatar hash in the database.
type UpdateAvatarRequestDB struct {
	RequestTime time.Time
	UserID      int
	AvatarHash  string
}
