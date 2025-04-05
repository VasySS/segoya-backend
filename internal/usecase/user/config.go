package user

import (
	"time"

	"github.com/VasySS/segoya-backend/internal/config"
)

// Config contains configuration for user usecase.
type Config struct {
	AvatarUpdateLimit time.Duration
}

// NewConfig returns a new local config from general config.
func NewConfig(conf config.Config) Config {
	return Config{
		AvatarUpdateLimit: conf.Limits.AvatarUpdateLimit,
	}
}
