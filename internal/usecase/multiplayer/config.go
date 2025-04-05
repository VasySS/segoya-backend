package multiplayer

import (
	"time"

	"github.com/VasySS/segoya-backend/internal/config"
)

// Config contains configuration for multiplayer usecase.
type Config struct {
	// Initial delay before round starts on frontend (to allow panorama to load).
	RoundStartDelay time.Duration
	// Delay after round has ended to start a new round (to allow users to see results).
	RoundEndDelay time.Duration
}

// NewConfig returns a new local config from general config.
func NewConfig(cfg config.Config) Config {
	return Config{
		RoundStartDelay: cfg.Limits.RoundStartDelay,
		RoundEndDelay:   cfg.Limits.RoundEndDelay,
	}
}
