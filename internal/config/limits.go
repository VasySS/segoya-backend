package config

import (
	"time"
)

// Limits contains various application limits - e.g. lobby expiration time.
type Limits struct {
	LobbyExpiration   time.Duration
	LobbyIDLength     int
	RoundStartDelay   time.Duration
	RoundEndDelay     time.Duration
	AvatarUpdateLimit time.Duration
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
}

func newLimits() Limits {
	return Limits{
		LobbyExpiration:   3 * time.Minute,
		LobbyIDLength:     16,
		RoundStartDelay:   5 * time.Second,
		RoundEndDelay:     10 * time.Second,
		AvatarUpdateLimit: 5 * time.Minute,
		AccessTokenTTL:    1 * time.Hour,
		RefreshTokenTTL:   31 * 24 * time.Hour,
	}
}
