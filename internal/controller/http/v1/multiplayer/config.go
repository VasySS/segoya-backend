package multiplayer

import "github.com/VasySS/segoya-backend/internal/config"

// Config contains configuration for multiplayer HTTP handlers.
type Config struct{}

// NewConfig creates and returns new local config from general config.
func NewConfig(_ config.Config) Config {
	return Config{}
}
