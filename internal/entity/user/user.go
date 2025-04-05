// Package user contains types for working with user data.
package user

import "time"

// PublicProfile contains information, that can be seen by other users.
type PublicProfile struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	Name         string    `json:"name"`
	RegisterDate time.Time `json:"registerDate"`
	AvatarHash   string    `json:"avatarHash"`
}

// PrivateProfile contains information, that can only be seen by the owner of the profile.
type PrivateProfile struct {
	PublicProfile
	Password         string    `json:"-"`
	AvatarLastUpdate time.Time `json:"-"`
}

// ToPublicProfile returns public information from PrivateProfile.
func (u PrivateProfile) ToPublicProfile() PublicProfile {
	return u.PublicProfile
}

// MultiplayerUser contains information about multiplayer user.
type MultiplayerUser struct {
	PublicProfile
	// Guessed   bool `json:"guessed"` // TODO - implement
	Connected bool `json:"connected"`
	Score     int  `json:"score"`
}
